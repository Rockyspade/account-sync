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
	user    *User
	client  *github.Client
	curOrgs map[string]*Organization
	ghOrgs  map[string]*github.Organization
}

func NewOrganizationSyncer(db *sqlx.DB, cfg *Config) *OrganizationSyncer {
	return &OrganizationSyncer{db: db, cfg: cfg}
}

func (osync *OrganizationSyncer) Sync(user *User, client *github.Client) error {
	ctx := &orgSyncContext{
		user:    user,
		client:  client,
		curOrgs: map[string]*Organization{},
		ghOrgs:  map[string]*github.Organization{},
	}
	curOrgs, err := osync.getCurrentlySyncedOrganizations(ctx)
	if err != nil {
		return err
	}

	for _, org := range curOrgs {
		ctx.curOrgs[org.Login.String] = org
	}

	ghOrgs, err := osync.getGithubOrgs(ctx)
	if err != nil {
		return err
	}

	for _, org := range ghOrgs {
		ctx.ghOrgs[*org.Login] = org
	}

	for _, org := range curOrgs {
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

func (osync *OrganizationSyncer) getGithubOrgs(ctx *orgSyncContext) ([]*github.Organization, error) {
	allOrgs := []*github.Organization{}
	listOpts := &github.ListOptions{Page: 1, PerPage: 100}

	for {
		ghOrgs, resp, err := ctx.client.Organizations.List("", listOpts)
		if err != nil {
			return allOrgs, err
		}

		for _, org := range ghOrgs {
			log.Printf("msg=\"fetching full org\" sync=organizations login=%v org=%v",
				ctx.user.Login.String, *org.Login)

			fullOrg, _, err := ctx.client.Organizations.Get(*org.Login)
			if err != nil {
				return allOrgs, err
			}
			allOrgs = append(allOrgs, fullOrg)
		}

		if resp.NextPage == 0 {
			break
		}

		listOpts.Page = resp.NextPage
	}

	return allOrgs, nil
}
