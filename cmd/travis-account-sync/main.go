package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/codegangsta/cli"
	"github.com/google/go-github/github"
	"github.com/jmoiron/sqlx"
	"github.com/travis-ci/encrypted-column"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v2"

	_ "github.com/lib/pq"
)

var (
	errInvalidGithubScopesYAML = fmt.Errorf("the GithubScopesYAML value is invalid")
	VersionString              = "0.1.0"
)

type tokenSource struct {
	token *oauth2.Token
}

func (ts *tokenSource) Token() (*oauth2.Token, error) {
	return ts.token, nil
}

type User struct {
	ID               int            `db:"id"`
	Name             sql.NullString `db:"name"`
	Login            sql.NullString `db:"login"`
	Email            sql.NullString `db:"email"`
	CreatedAt        *time.Time     `db:"created_at"`
	UpdatedAt        *time.Time     `db:"updated_at"`
	IsAdmin          bool           `db:"is_admin"`
	GithubID         int            `db:"github_id"`
	GithubOauthToken sql.NullString `db:"github_oauth_token"`
	GravatarID       sql.NullString `db:"gravatar_id"`
	IsSyncing        bool           `db:"is_syncing"`
	Locale           sql.NullString `db:"locale"`
	SyncedAt         *time.Time     `db:"synced_at"`
	GithubScopesYAML sql.NullString `db:"github_scopes"`
	Education        bool           `db:"education"`

	GithubScopes []string
}

func (user *User) Hydrate() error {
	if user.GithubScopes != nil {
		return nil
	}

	user.GithubScopes = []string{}

	if !user.GithubScopesYAML.Valid {
		return errInvalidGithubScopesYAML
	}

	return yaml.Unmarshal([]byte(user.GithubScopesYAML.String), &user.GithubScopes)
}

func (user *User) Clone() *User {
	return &User{
		ID:               user.ID,
		Name:             user.Name,
		Login:            user.Login,
		Email:            user.Email,
		CreatedAt:        user.CreatedAt,
		UpdatedAt:        user.UpdatedAt,
		IsAdmin:          user.IsAdmin,
		GithubID:         user.GithubID,
		GithubOauthToken: user.GithubOauthToken,
		GravatarID:       user.GravatarID,
		IsSyncing:        user.IsSyncing,
		Locale:           user.Locale,
		SyncedAt:         user.SyncedAt,
		GithubScopesYAML: user.GithubScopesYAML,
		Education:        user.Education,
		GithubScopes:     user.GithubScopes,
	}
}

type userSyncError struct {
	TravisLogin    string
	TravisGithubID int
	GithubLogin    string
	GithubID       int
}

func (err *userSyncError) Error() string {
	return fmt.Sprintf("msg=\"updating user failed because of mismatched data\" "+
		"travis_github_id=%v github_id=%v travis_login=%v github_login=%v",
		err.TravisGithubID, err.GithubID, err.TravisLogin, err.GithubLogin)
}

type Syncer struct {
	db                       *sqlx.DB
	EducationEndpointTimeout int
}

func (syncer *Syncer) runSync(c *cli.Context) {
	log.SetFlags(log.LstdFlags)

	encryptionKeyHex := c.String("encryption-key")
	if encryptionKeyHex == "" {
		log.Fatal("msg=\"missing encryption key\"")
	}
	githubUsernames := []string{}
	for _, username := range c.StringSlice("github-usernames") {
		if strings.TrimSpace(username) == "" {
			continue
		}
		githubUsernames = append(githubUsernames, username)
	}

	log.Println("msg=\"connecting to database\"")
	db, err := sqlx.Connect("postgres", c.String("database-url"))
	if err != nil {
		log.Fatal(err)
	}

	syncer.db = db

	ghTokCol, err := encryptedcolumn.NewEncryptedColumn(encryptionKeyHex, true)
	if err != nil {
		log.Fatal(err)
	}

	errMap := map[string][]error{}

	for _, githubUsername := range githubUsernames {
		user := &User{}

		errMap[githubUsername] = []error{}
		addErr := func(err error) {
			errMap[githubUsername] = append(errMap[githubUsername], err)
		}

		log.Printf("msg=\"fetching user\" login=%q", githubUsername)
		err = db.Get(user, "SELECT * FROM users WHERE login = $1", githubUsername)
		if err != nil {
			addErr(err)
			continue
		}

		err = user.Hydrate()
		if err != nil {
			addErr(err)
			continue
		}

		token, err := ghTokCol.Load(user.GithubOauthToken.String)
		if err != nil {
			addErr(err)
			continue
		}

		ts := &tokenSource{token: &oauth2.Token{AccessToken: token}}

		log.Printf("msg=\"creating oauth2 client\" token=%q", ts.token.AccessToken)
		tc := oauth2.NewClient(oauth2.NoContext, ts)
		client := github.NewClient(tc)
		client.UserAgent = fmt.Sprintf("Travis CI Account Sync/%s", VersionString)

		err = syncer.syncUser(user, client)
		if err != nil {
			addErr(err)
			continue
		}

		err = syncer.syncOrganizations(user, client)
		if err != nil {
			addErr(err)
			continue
		}

		err = syncer.syncOwnerRepositories(user, client)
		if err != nil {
			addErr(err)
			continue
		}
	}

	for githubUsername, errors := range errMap {
		for _, err := range errors {
			log.Printf("level=error login=%s err=%q", githubUsername, err.Error())
		}
	}
}

func (syncer *Syncer) syncUser(user *User, client *github.Client) error {
	ghUser, _, err := client.Users.Get(user.Login.String)
	if err != nil {
		return err
	}

	err = &userSyncError{
		TravisLogin:    user.Login.String,
		TravisGithubID: user.GithubID,
		GithubLogin:    *ghUser.Login,
		GithubID:       *ghUser.ID,
	}

	if user.GithubID != *ghUser.ID {
		return err
	}

	if user.Login.String != *ghUser.Login {
		return err
	}

	newUser := user.Clone()

	email, err := syncer.getUserEmail(user, ghUser, client)
	if err != nil {
		return err
	}

	edu, err := syncer.getIsEducation(user, ghUser, client)
	if err != nil {
		return err
	}

	newUser.Name = sql.NullString{String: *ghUser.Name, Valid: true}
	newUser.Login = sql.NullString{String: *ghUser.Login, Valid: true}
	newUser.GravatarID = sql.NullString{String: *ghUser.GravatarID, Valid: true}
	newUser.Email = sql.NullString{String: email, Valid: true}
	if edu != nil {
		newUser.Education = *edu
	}

	log.Printf("msg=\"would be saving user\" user=%#v", newUser)

	return nil
}

func (syncer *Syncer) getUserEmail(user *User, ghUser *github.User, client *github.Client) (string, error) {
	email := *ghUser.Email
	if email != "" {
		return email, nil
	}

	allEmails, err := syncer.getAllEmails(user, client)
	if err != nil {
		return "", err
	}

	primaryEmail := ""
	firstEmail := ""
	verifiedEmail := ""
	for i, email := range allEmails {
		if i == 0 {
			firstEmail = *email.Email
		}

		if verifiedEmail == "" && email.Verified != nil && *email.Verified {
			verifiedEmail = *email.Email
		}

		if primaryEmail == "" && email.Primary != nil && *email.Primary {
			primaryEmail = *email.Email
		}
	}

	if primaryEmail != "" {
		return primaryEmail, nil
	}

	if verifiedEmail != "" {
		return verifiedEmail, nil
	}

	if user.Email.String != "" {
		return user.Email.String, nil
	}

	return firstEmail, nil
}

func (syncer *Syncer) getAllEmails(user *User, client *github.Client) ([]github.UserEmail, error) {
	if !sliceContains(user.GithubScopes, "user") && !sliceContains(user.GithubScopes, "user:email") {
		return []github.UserEmail{}, nil
	}

	emails, _, err := client.Users.ListEmails(&github.ListOptions{
		Page:    1,
		PerPage: 100,
	})
	return emails, err
}

func (syncer *Syncer) getIsEducation(user *User, ghUser *github.User, client *github.Client) (*bool, error) {
	req, err := client.NewRequest("GET", "https://education.github.com/api/user", nil)
	if err != nil {
		return nil, err
	}

	body := map[string]bool{"student": false}

	_, err = client.Do(req, &body)
	if err != nil {
		return nil, err
	}

	student := body["student"]

	return &student, nil
}

func (syncer *Syncer) syncOrganizations(user *User, client *github.Client) error {
	return nil
}

func (syncer *Syncer) syncOwnerRepositories(user *User, client *github.Client) error {
	// TODO: 100% less hardcoding
	startPage := 1
	curPage := startPage
	for {
		opts := &github.RepositoryListOptions{
			Type: "public",
			ListOptions: github.ListOptions{
				PerPage: 100,
				Page:    curPage,
			},
		}

		log.Printf("msg=\"fetching repositories\" page=%v login=%q", curPage, user.Login.String)
		repos, response, err := client.Repositories.List(user.Login.String, opts)
		if err != nil {
			return err
		}

		for _, repo := range repos {
			fmt.Printf("id=%v full_name=%v\n", *repo.ID, *repo.FullName)
		}

		if response.NextPage == 0 {
			break
		}

		curPage += 1
	}
	return nil
}

func sliceContains(sl []string, s string) bool {
	for _, candidate := range sl {
		if candidate == s {
			return true
		}
	}

	return false
}

func main() {
	app := cli.NewApp()
	app.Usage = "Syncing accounts"
	app.Version = VersionString
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "k, encryption-key",
			Value:  "",
			EnvVar: "TRAVIS_ACCOUNT_SYNC_ENCRYPTION_KEY",
		},
		cli.StringFlag{
			Name:   "d, database-url",
			Value:  "",
			EnvVar: "TRAVIS_ACCOUNT_SYNC_DATABASE_URL",
		},
		cli.StringSliceFlag{
			Name:   "u, github-usernames",
			Value:  &cli.StringSlice{},
			EnvVar: "TRAVIS_ACCOUNT_SYNC_GITHUB_USERNAMES",
		},
		cli.IntFlag{
			Name:   "education-endpoint-timeout",
			Value:  5000,
			EnvVar: "TRAVIS_ACCOUNT_SYNC_EDUCATION_ENDPOINT_TIMEOUT",
		},
	}
	app.Action = func(c *cli.Context) {
		syncer := &Syncer{
			EducationEndpointTimeout: c.Int("education-endpoint-timeout"),
		}
		syncer.runSync(c)
	}
	app.Run(os.Args)
}
