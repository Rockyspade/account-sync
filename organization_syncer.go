package accountsync

import (
	"log"

	"github.com/google/go-github/github"
	"github.com/jmoiron/sqlx"
)

type OrganizationSyncer struct {
	db  *sqlx.DB
	cfg *Config
}

type orgSyncContext struct {
	user   *User
	client *github.Client
}

func NewOrganizationSyncer(db *sqlx.DB, cfg *Config) *OrganizationSyncer {
	return &OrganizationSyncer{db: db, cfg: cfg}
}

func (osync *OrganizationSyncer) Sync(user *User, client *github.Client) error {
	ctx := &orgSyncContext{user: user, client: client}
	currentOrgs, err := osync.getCurrentlySyncedOrganizations(ctx)
	if err != nil {
		return err
	}

	for _, org := range currentOrgs {
		log.Printf("sync=organizations login=%s org=%s", user.Login.String, org.Login.String)
	}
	return nil
}

func (osync *OrganizationSyncer) getCurrentlySyncedOrganizations(ctx *orgSyncContext) ([]*Organization, error) {
	orgs := []*Organization{}
	err := osync.db.Select(&orgs, `
		SELECT *
		FROM organizations
		WHERE id IN (
			SELECT organization_id
			FROM memberships
			WHERE user_id = $1
		)`, ctx.user.ID)
	return orgs, err
}
