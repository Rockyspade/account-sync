package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
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
	GithubScopes     sql.NullString `db:"github_scopes"`
	Education        bool           `db:"education"`
}

func main() {
	log.SetFlags(log.LstdFlags)

	encryptionKeyHex := os.Getenv("TRAVIS_ACCOUNT_SYNC_ENCRYPTION_KEY")
	if encryptionKeyHex == "" {
		log.Fatal("msg=\"missing TRAVIS_ACCOUNT_SYNC_ENCRYPTION_KEY\"")
	}
	githubUsernames := []string{}
	for i, username := range os.Args {
		if i == 0 {
			continue
		}
		githubUsernames = append(githubUsernames, username)
	}
	for _, username := range strings.Split(os.Getenv("GITHUB_USERNAMES"), ",") {
		if strings.TrimSpace(username) == "" {
			continue
		}
		githubUsernames = append(githubUsernames, username)
	}

	log.Println("msg=\"connecting to database\"")
	db, err := sqlx.Connect("postgres", os.Getenv("TRAVIS_ACCOUNT_SYNC_DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}

	ghTokCol, err := encryptedcolumn.NewEncryptedColumn(encryptionKeyHex, true)
	if err != nil {
		log.Fatal(err)
	}

	for _, githubUsername := range githubUsernames {
		user := &User{}
		log.Printf("msg=\"fetching user\" login=%q", githubUsername)
		err = db.Get(user, "SELECT * FROM users WHERE login = $1", githubUsername)
		if err != nil {
			log.Fatal(err)
		}

		tok, err := ghTokCol.Load(user.GithubOauthToken.String)
		if err != nil {
			log.Printf("level=error msg=\"unable to decrypt github oauth token\" err=\"%v\"", err)
			continue
		}

		ts := &tokenSource{token: &oauth2.Token{AccessToken: tok}}

		log.Printf("msg=\"creating oauth2 client\" token=%q", ts.token.AccessToken)
		tc := oauth2.NewClient(oauth2.NoContext, ts)
		client := github.NewClient(tc)

		opts := &github.RepositoryListOptions{
			Type: "public",
			ListOptions: github.ListOptions{
				PerPage: 100,
				Page:    1,
			},
		}

		log.Printf("msg=\"fetching repositories\" login=%q", githubUsername)
		repos, _, err := client.Repositories.List(githubUsername, opts)
		if err != nil {
			log.Fatal(err)
		}

		for _, repo := range repos {
			fmt.Printf("id=%v full_name=%v\n", *repo.ID, *repo.FullName)
		}
	}
}
