package accountsync

import (
	"github.com/google/go-github/github"
	"github.com/jmoiron/sqlx"
)

type OrganizationSyncer struct {
	db  *sqlx.DB
	cfg *Config
}

func NewOrganizationSyncer(db *sqlx.DB, cfg *Config) *OrganizationSyncer {
	return &OrganizationSyncer{db: db, cfg: cfg}
}

func (os *OrganizationSyncer) Sync(user *User, client *github.Client) error {
	return nil
}
