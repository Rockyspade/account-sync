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
	userInfoSyncer := NewUserInfoSyncer(db, syncer.cfg)
	orgSyncer := NewOrganizationSyncer(db, syncer.cfg)
	ownerReposSyncer := NewOwnerRepositoriesSyncer(db, syncer.cfg)

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
		err = userInfoSyncer.Sync(user, client)
		if err != nil {
			addErr(err)
			log.Printf("state=errored sync=user_info err=%v login=%v", err, githubUsername)
			continue
		}
		log.Printf("state=completed sync=user_info login=%v duration=%v",
			githubUsername, time.Now().UTC().Sub(started))

		started = time.Now().UTC()
		log.Printf("state=started sync=organizations login=%v", githubUsername)
		err = orgSyncer.Sync(user, client)
		if err != nil {
			addErr(err)
			log.Printf("state=errored sync=organizations err=%v login=%v", err, githubUsername)
			continue
		}
		log.Printf("state=completed sync=organizations login=%v duration=%v",
			githubUsername, time.Now().UTC().Sub(started))

		started = time.Now().UTC()
		log.Printf("state=started sync=repositories login=%v", githubUsername)
		err = ownerReposSyncer.Sync(user, client)
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
