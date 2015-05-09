package accountsync

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"github.com/jmoiron/sqlx"
	"github.com/travis-ci/encrypted-column"
	"golang.org/x/oauth2"

	_ "github.com/lib/pq"
)

type tokenSource struct {
	token *oauth2.Token
}

func (ts *tokenSource) Token() (*oauth2.Token, error) {
	return ts.token, nil
}

type UserSyncError struct {
	TravisLogin    string
	TravisGithubID int
	GithubLogin    string
	GithubID       int
}

func (err *UserSyncError) Error() string {
	return fmt.Sprintf("msg=\"updating user failed because of mismatched data\" "+
		"travis_github_id=%v github_id=%v travis_login=%v github_login=%v",
		err.TravisGithubID, err.GithubID, err.TravisLogin, err.GithubLogin)
}

type Syncer struct {
	db  *sqlx.DB
	cfg *Config
}

func NewSyncer(cfg *Config) *Syncer {
	return &Syncer{cfg: cfg}
}

func (syncer *Syncer) Sync() {
	log.SetFlags(log.LstdFlags)

	if syncer.cfg.EncryptionKey == "" {
		log.Fatal("msg=\"missing encryption key\"")
	}

	log.Println("msg=\"connecting to database\"")
	db, err := sqlx.Connect("postgres", syncer.cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}

	syncer.db = db

	ghTokCol, err := encryptedcolumn.NewEncryptedColumn(syncer.cfg.EncryptionKey, true)
	if err != nil {
		log.Fatal(err)
	}

	errMap := map[string][]error{}

	for _, githubUsername := range syncer.cfg.GithubUsernames {
		if strings.TrimSpace(githubUsername) == "" {
			continue
		}
		user := &User{}

		errMap[githubUsername] = []error{}
		addErr := func(err error) {
			errMap[githubUsername] = append(errMap[githubUsername], err)
		}

		log.Printf("msg=\"fetching user\" login=%v", githubUsername)
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

		tc := oauth2.NewClient(oauth2.NoContext, ts)
		client := github.NewClient(tc)
		client.UserAgent = fmt.Sprintf("Travis CI Account Sync/%s", VersionString)

		fullStarted := time.Now().UTC()
		log.Printf("state=started sync=user login=%v", githubUsername)

		started := time.Now().UTC()
		log.Printf("state=started sync=user_info login=%v", githubUsername)
		err = syncer.syncUser(user, client)
		if err != nil {
			addErr(err)
			log.Printf("state=errored sync=user_info err=%v login=%v", err, githubUsername)
			continue
		}
		log.Printf("state=completed sync=user_info login=%v duration=%v",
			githubUsername, time.Now().UTC().Sub(started))

		started = time.Now().UTC()
		log.Printf("state=started sync=organizations login=%v", githubUsername)
		err = syncer.syncOrganizations(user, client)
		if err != nil {
			addErr(err)
			log.Printf("state=errored sync=organizations err=%v login=%v", err, githubUsername)
			continue
		}
		log.Printf("state=completed sync=organizations login=%v duration=%v",
			githubUsername, time.Now().UTC().Sub(started))

		started = time.Now().UTC()
		log.Printf("state=started sync=repositories login=%v", githubUsername)
		err = syncer.syncOwnerRepositories(user, client)
		if err != nil {
			addErr(err)
			continue
		}
		log.Printf("state=completed sync=repositories login=%v duration=%v",
			githubUsername, time.Now().UTC().Sub(started))

		log.Printf("state=completed sync=user login=%v duration=%v",
			githubUsername, time.Now().UTC().Sub(fullStarted))
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

	syncErr := &UserSyncError{
		TravisLogin:    user.Login.String,
		TravisGithubID: user.GithubID,
		GithubLogin:    *ghUser.Login,
		GithubID:       *ghUser.ID,
	}

	if user.GithubID != *ghUser.ID {
		return syncErr
	}

	if user.Login.String != *ghUser.Login {
		return syncErr
	}

	email, err := syncer.getUserEmail(user, ghUser, client)
	if err != nil {
		return err
	}

	edu, err := syncer.getIsEducation(user, ghUser, client)
	if err != nil {
		return err
	}

	isEdu := user.Education
	if edu != nil {
		isEdu = *edu
	}

	tx, err := syncer.db.Beginx()
	if err != nil {
		return err
	}

	log.Printf("msg=\"updating user info\" sync=user_info login=%v", user.Login.String)
	_, err = tx.Exec(`
		UPDATE users
		SET name = $1, login = $2, gravatar_id = $3, email = $4, education = $5
		WHERE id = $6
	`, *ghUser.Name, *ghUser.Login, *ghUser.GravatarID, email, isEdu,
		user.ID)

	if err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
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

		log.Printf("msg=\"fetching repositories\" page=%v login=%v",
			curPage, user.Login.String)
		repos, response, err := client.Repositories.List(user.Login.String, opts)
		if err != nil {
			return err
		}

		for _, repo := range repos {
			log.Printf("login=%v repo_id=%v repo_full_name=%v\n",
				user.Login.String, *repo.ID, *repo.FullName)
		}

		if response.NextPage == 0 {
			break
		}

		curPage += 1
	}
	return nil
}
