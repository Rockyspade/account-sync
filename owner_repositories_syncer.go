package accountsync

import (
	"log"

	"github.com/google/go-github/github"
	"github.com/jmoiron/sqlx"
)

type OwnerRepositoriesSyncer struct {
	db  *sqlx.DB
	cfg *Config
}

func NewOwnerRepositoriesSyncer(db *sqlx.DB, cfg *Config) *OwnerRepositoriesSyncer {
	return &OwnerRepositoriesSyncer{db: db, cfg: cfg}
}

func (ors *OwnerRepositoriesSyncer) Sync(user *User, client *github.Client) error {
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
