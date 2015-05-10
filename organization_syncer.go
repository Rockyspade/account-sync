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

	err := user.HydrateOrganizations(osync.db)
	if err != nil {
		return err
	}

	for _, org := range user.Organizations {
		ctx.curOrgs[org.Login.String] = org
	}

	ghOrgs, err := osync.getGithubOrgs(ctx)
	if err != nil {
		return err
	}

	for _, org := range ghOrgs {
		ctx.ghOrgs[*org.Login] = org
	}

	for _, org := range user.Organizations {
		log.Printf("sync=organizations login=%s org=%s", user.Login.String, org.Login.String)
		// TODO: create or update org
		// TODO: create new memberships
		// TODO: remove outdated memberships
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
			log.Printf("msg=\"fetching full org\" sync=organizations login=%v page=%v org=%v",
				ctx.user.Login.String, listOpts.Page, *org.Login)

			fullOrg, _, err := ctx.client.Organizations.Get(*org.Login)
			if err != nil {
				return allOrgs, err
			}

			if *fullOrg.PublicRepos > osync.cfg.OrganizationsRepositoriesLimit {
				log.Printf("msg=\"skipping org\" sync=organizations login=%v page=%v org=%v public_repos=%v public_repos_limit=%v",
					ctx.user.Login.String, listOpts.Page, *org.Login, *org.PublicRepos,
					osync.cfg.OrganizationsRepositoriesLimit)
				continue
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
