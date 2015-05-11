package accountsync

import (
	"fmt"
	"strings"

	"github.com/google/go-github/github"
	"github.com/jmoiron/sqlx"
)

type OwnerRepositoriesSyncer struct {
	db  *sqlx.DB
	cfg *Config
}

type ownerRepoSyncContext struct {
	user   *User
	client *github.Client
}

type errOrgSync struct {
	errMap *map[string][]error
}

func (eos *errOrgSync) Error() string {
	if eos.errMap == nil {
		return ""
	}

	s := []string{}
	for org, errors := range *eos.errMap {
		for _, err := range errors {
			s = append(s, fmt.Sprintf("%v:%v", org, err))
		}
	}

	return strings.Join(s, "; ")
}

func NewOwnerRepositoriesSyncer(db *sqlx.DB, cfg *Config) *OwnerRepositoriesSyncer {
	return &OwnerRepositoriesSyncer{db: db, cfg: cfg}
}

func (ors *OwnerRepositoriesSyncer) Sync(user *User, client *github.Client) error {
	ctx := &ownerRepoSyncContext{
		user:   user,
		client: client,
	}

	err := user.HydrateOrganizations(ors.db)
	if err != nil {
		return err
	}

	owners := []*Owner{
		&Owner{
			Type: "user",
			User: user,
		},
	}

	for _, org := range user.Organizations {
		owners = append(owners,
			&Owner{
				Type:         "organization",
				Organization: org,
			})
	}

	hadRepoSyncErr := false
	githubRepoIDs := []*int{}

	orgSyncErrors := map[string][]error{}

	for _, owner := range owners {
		rs := NewRepositoriesSyncer(ors.db, ors.cfg)
		repoIDs, err := rs.Sync(owner, user, client)
		if err != nil {
			hadRepoSyncErr = true
			key := owner.Key()
			if _, ok := orgSyncErrors[key]; !ok {
				orgSyncErrors[key] = []error{}
			}
			orgSyncErrors[key] = append(orgSyncErrors[key], err)
			continue
		}

		if repoIDs != nil {
			githubRepoIDs = append(githubRepoIDs, repoIDs...)
		}
	}

	err = ors.cleanupRepos(githubRepoIDs, ctx)
	if err != nil {
		return err
	}

	if hadRepoSyncErr {
		return &errOrgSync{errMap: &orgSyncErrors}
	}

	return nil
}

func (ors *OwnerRepositoriesSyncer) cleanupRepos(githubRepoIDs []*int, ctx *ownerRepoSyncContext) error {
	// TODO: find old repos and revoke all permissions for the user
	return nil
}
