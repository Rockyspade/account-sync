package accountsync

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/google/go-github/github"
	"github.com/jmoiron/sqlx"
)

type repoSyncContext struct {
	owner  *Owner
	user   *User
	client *github.Client
}

type RepositoriesSyncer struct {
	db  *sqlx.DB
	cfg *Config
}

func NewRepositoriesSyncer(db *sqlx.DB, cfg *Config) *RepositoriesSyncer {
	return &RepositoriesSyncer{
		db:  db,
		cfg: cfg,
	}
}

func (rs *RepositoriesSyncer) Sync(owner *Owner, user *User, client *github.Client) ([]*int, error) {
	ctx := &repoSyncContext{
		owner:  owner,
		user:   user,
		client: client,
	}
	githubRepoIDs := []*int{}

	for _, syncType := range rs.cfg.SyncTypes {
		syncTypeGithubIDs, err := rs.syncReposOfType(syncType, ctx)
		if err != nil {
			return githubRepoIDs, err
		}
		githubRepoIDs = append(githubRepoIDs, syncTypeGithubIDs...)
	}

	return githubRepoIDs, nil
}
func (rs *RepositoriesSyncer) syncReposOfType(syncType string, ctx *repoSyncContext) ([]*int, error) {
	curPage := rs.cfg.RepositoriesStartPage

	for {
		opts := &github.RepositoryListOptions{
			Type: syncType,
			ListOptions: github.ListOptions{
				PerPage: 100,
				Page:    curPage,
			},
		}

		log.Printf("sync=repositories page=%v owner=%v login=%v",
			curPage, ctx.owner, ctx.user.Login.String)

		var (
			repos    []github.Repository
			response *github.Response
			err      error
		)

		switch ctx.owner.Type {
		case "user":
			repos, response, err = rs.getUserRepositories(opts, ctx)
		case "organization":
			repos, response, err = rs.getOrganizationRepositories(opts, ctx)
		default:
			panic(fmt.Errorf("invalid owner type %q", ctx.owner.Type))
		}

		if err != nil {
			log.Printf("level=error sync=repositories page=%v owner=%v login=%v err=%v",
				curPage, ctx.owner, ctx.user.Login.String, err)
			continue
		}

		for _, repo := range repos {
			err = rs.syncRepo(&repo, ctx)
			if err != nil {
				log.Printf("level=error sync=repository repo_id=%v login=%v repo=%v err=%v",
					*repo.ID, ctx.user.Login.String, *repo.FullName, err)
			}
		}

		if response.NextPage == 0 {
			break
		}

		curPage += 1
	}
	return nil, nil
}

func (rs *RepositoriesSyncer) getUserRepositories(opts *github.RepositoryListOptions, ctx *repoSyncContext) ([]github.Repository, *github.Response, error) {
	return ctx.client.Repositories.List("", opts)
}

func (rs *RepositoriesSyncer) getOrganizationRepositories(opts *github.RepositoryListOptions, ctx *repoSyncContext) ([]github.Repository, *github.Response, error) {
	repos := []github.Repository{}
	reqURL := fmt.Sprintf("/organizations/%v/repos?page=%v&per_page=%v&type=%s",
		ctx.owner.Organization.GithubID, opts.ListOptions.Page, opts.ListOptions.PerPage, opts.Type)
	req, err := ctx.client.NewRequest("GET", reqURL, nil)
	if err != nil {
		return repos, nil, err
	}

	response, err := ctx.client.Do(req, &repos)
	if err != nil {
		return repos, response, err
	}

	return repos, response, err
}

func (rs *RepositoriesSyncer) shouldSync(repo *github.Repository) bool {
	t := "public"
	if *repo.Private {
		t = "private"
	}
	return sliceContains(rs.cfg.SyncTypes, t)
}

func (rs *RepositoriesSyncer) syncRepo(ghRepo *github.Repository, ctx *repoSyncContext) error {
	log.Printf("sync=repository repo_id=%v login=%v repo=%v\n",
		*ghRepo.ID, ctx.user.Login.String, *ghRepo.FullName)
	if !rs.shouldSync(ghRepo) {
		log.Printf("msg=\"skipping\" sync=repository repo_id=%v login=%v repo=%v\n",
			*ghRepo.ID, ctx.user.Login.String, *ghRepo.FullName)
		return nil
	}

	started := time.Now().UTC()
	log.Printf("state=started sync=repository repo_id=%v login=%v repo=%v",
		*ghRepo.ID, ctx.user.Login.String, *ghRepo.FullName)

	owner, err := rs.findRepoOwner(ghRepo, ctx)
	if err != nil {
		return err
	}

	if owner == nil {
		owner, err = rs.createRepoOwner(ghRepo, ctx)
	}

	repo, err := rs.findRepoByGithubID(*ghRepo.ID, ctx)
	if err != nil {
		return err
	}

	if repo == nil {
		log.Printf("action=creating sync=repository repo_id=%v login=%v repo=%v",
			*ghRepo.ID, ctx.user.Login.String, *ghRepo.FullName)
		repo, err = rs.createRepo(ghRepo, ctx)
		if err != nil {
			return err
		}
	} else {
		log.Printf("action=updating sync=repository repo_id=%v login=%v repo=%v",
			*ghRepo.ID, ctx.user.Login.String, *ghRepo.FullName)
		repo.UpdateFromGithubRepository(ghRepo)
		repo, err = rs.updateRepo(repo, ctx)
		if err != nil {
			return err
		}
	}

	// TODO: sync permissions if present
	// TODO: permit if permittable

	if err != nil {
		return err
	}

	log.Printf("state=completed sync=repository repo_id=%v login=%v repo=%v duration=%v",
		*ghRepo.ID, ctx.user.Login.String, *ghRepo.FullName, time.Now().UTC().Sub(started))
	return nil
}

func (rs *RepositoriesSyncer) findRepoOwner(ghRepo *github.Repository, ctx *repoSyncContext) (*Owner, error) {
	owner := &Owner{}

	log.Printf("level=debug sync=repository msg=\"finding user\" github_id=%v", *ghRepo.Owner.ID)
	user, err := rs.findUserByGithubID(*ghRepo.Owner.ID)
	if err != nil {
		return nil, err
	}

	if user != nil {
		owner.Type = "user"
		owner.User = user
		return owner, nil
	}

	log.Printf("level=debug sync=repository msg=\"finding org\" github_id=%v", *ghRepo.Owner.ID)
	org, err := rs.findOrgByGithubID(*ghRepo.Owner.ID)
	if err != nil {
		return nil, err
	}

	if org != nil {
		owner.Type = "organization"
		owner.Organization = org
		return owner, nil
	}

	return nil, nil
}

func (rs *RepositoriesSyncer) findRepoByGithubID(ghRepoID int, ctx *repoSyncContext) (*Repository, error) {
	repo := &Repository{}
	err := rs.db.Get(repo, `SELECT * FROM repositories WHERE github_id = $1`, ghRepoID)
	if err == sql.ErrNoRows {
		repo = nil
		err = nil
	}
	return repo, err
}

func (rs *RepositoriesSyncer) createRepo(ghRepo *github.Repository, ctx *repoSyncContext) (*Repository, error) {
	now := time.Now().UTC()
	repo := &Repository{
		CreatedAt: &now,
		UpdatedAt: &now,
	}
	repo.UpdateFromGithubRepository(ghRepo)

	res, err := rs.db.NamedExec(`
		INSERT INTO repositories (
			created_at,
			default_branch,
			description,
			github_id,
			github_language,
			name,
			owner_id,
			owner_name,
			owner_type,
			private,
			url,
			updated_at
		) VALUES (
			:created_at,
			:default_branch,
			:description,
			:github_id,
			:github_language,
			:name,
			:owner_id,
			:owner_name,
			:owner_type,
			:private,
			:url,
			:updated_at
		) RETURNING id
	`, repo)
	if err != nil {
		return nil, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	repo.ID = id

	return repo, nil
}

func (rs *RepositoriesSyncer) updateRepo(repo *Repository, ctx *repoSyncContext) (*Repository, error) {
	now := time.Now().UTC()
	repo.UpdatedAt = &now
	_, err := rs.db.NamedExec(`
		UPDATE repositories
		SET
			default_branch = :default_branch
			description = :description
			github_id = :github_id
			github_language = :github_language
			name = :name
			owner_id = :owner_id
			owner_name = :owner_name
			owner_type = :owner_type
			private = :private
			url = :url
			updated_at = :updated_at
		WHERE id = :id
	`, repo)
	return repo, err
}

func (rs *RepositoriesSyncer) findUserByGithubID(ghUserID int) (*User, error) {
	user := &User{}
	err := rs.db.Get(user, `SELECT * FROM users WHERE github_id = $1`, ghUserID)
	if err == sql.ErrNoRows {
		user = nil
		err = nil
	}
	return user, err
}

func (rs *RepositoriesSyncer) findOrgByGithubID(ghOrgID int) (*Organization, error) {
	org := &Organization{}
	err := rs.db.Get(org, `SELECT * FROM organizations WHERE github_id = $1`, ghOrgID)
	if err == sql.ErrNoRows {
		org = nil
		err = nil
	}
	return org, err
}

func (rs *RepositoriesSyncer) createRepoOwner(repo *github.Repository, ctx *repoSyncContext) (*Owner, error) {
	switch *repo.Owner.Type {
	case "User":
		ghUser, err := rs.getGithubUserByID(*repo.Owner.ID, ctx)
		if err != nil {
			return nil, err
		}
		user, err := rs.createUserFromGithubUser(ghUser, ctx)
		if err != nil {
			return nil, err
		}
		owner := &Owner{
			Type: "user",
			User: user,
		}
		log.Printf("level=warn login=%v id=%v sync=repository slug=%v status=created_user reason=owner_not_found",
			user.Login, user.ID, *repo.FullName)
		return owner, nil
	case "Organization":
		ghOrg, err := rs.getGithubOrgByID(*repo.Owner.ID, ctx)
		if err != nil {
			return nil, err
		}
		org, err := rs.createOrgFromGithubOrg(ghOrg, ctx)
		if err != nil {
			return nil, err
		}
		owner := &Owner{
			Type:         "organization",
			Organization: org,
		}
		log.Printf("level=warn login=%v id=%v sync=repository slug=%v status=created_org reason=owner_not_found",
			org.Login, org.ID, *repo.FullName)
		return owner, nil
	}

	return nil, nil
}

func (rs *RepositoriesSyncer) getGithubUserByID(userID int, ctx *repoSyncContext) (*github.User, error) {
	reqURL := fmt.Sprintf("/user/%v", userID)
	req, err := ctx.client.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	user := &github.User{}
	_, err = ctx.client.Do(req, user)
	if err != nil {
		return user, err
	}

	return user, err
}

func (rs *RepositoriesSyncer) getGithubOrgByID(orgID int, ctx *repoSyncContext) (*github.Organization, error) {
	reqURL := fmt.Sprintf("/organizations/%v", orgID)
	req, err := ctx.client.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	org := &github.Organization{}
	_, err = ctx.client.Do(req, org)
	if err != nil {
		return org, err
	}

	return org, err
}

func (rs *RepositoriesSyncer) createUserFromGithubUser(ghUser *github.User, ctx *repoSyncContext) (*User, error) {
	return nil, nil
}

func (rs *RepositoriesSyncer) createOrgFromGithubOrg(ghOrg *github.Organization, ctx *repoSyncContext) (*Organization, error) {
	return nil, nil
}
