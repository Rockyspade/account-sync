package accountsync

import (
	"database/sql"
	"time"

	"github.com/google/go-github/github"
)

type Repository struct {
	ID int64 `db:"id"`

	Active              sql.NullBool   `db:"active"`
	CreatedAt           *time.Time     `db:"created_at"`
	DefaultBranch       sql.NullString `db:"default_branch"`
	Description         sql.NullString `db:"description"`
	GithubID            sql.NullInt64  `db:"github_id"`
	GithubLanguage      sql.NullString `db:"github_language"`
	LastBuildDuration   sql.NullInt64  `db:"last_build_duration"`
	LastBuildFinishedAt *time.Time     `db:"last_build_finished_at"`
	LastBuildID         sql.NullInt64  `db:"last_build_id"`
	LastBuildNumber     sql.NullString `db:"last_build_number"`
	LastBuildState      sql.NullString `db:"last_build_state"`
	LastBuildStartedAt  *time.Time     `db:"last_build_started_at"`
	LastSync            *time.Time     `db:"last_sync"`
	Name                sql.NullString `db:"name"`
	NextBuildNumber     sql.NullString `db:"next_build_number"`
	OwnerEmail          sql.NullString `db:"owner_email"`
	OwnerID             sql.NullInt64  `db:"owner_id"`
	OwnerName           sql.NullString `db:"owner_name"`
	OwnerType           sql.NullString `db:"owner_type"`
	Private             sql.NullBool   `db:"private"`
	Settings            sql.NullString `db:"settings"`
	URL                 sql.NullString `db:"url"`
	UpdatedAt           *time.Time     `db:"updated_at"`
}

func (repo *Repository) UpdateFromGithubRepository(ghRepo *github.Repository) {
	repo.DefaultBranch = sql.NullString{String: strPtrOrEmpty(ghRepo.DefaultBranch), Valid: true}
	repo.Description = sql.NullString{String: strPtrOrEmpty(ghRepo.Description), Valid: true}
	repo.GithubID = sql.NullInt64{Int64: int64(*ghRepo.ID), Valid: true}
	repo.GithubLanguage = sql.NullString{String: strPtrOrEmpty(ghRepo.Language), Valid: true}
	repo.Name = sql.NullString{String: strPtrOrEmpty(ghRepo.Name), Valid: true}
	repo.OwnerID = sql.NullInt64{Int64: int64(*ghRepo.Owner.ID), Valid: true}
	repo.OwnerName = sql.NullString{String: strPtrOrEmpty(ghRepo.Owner.Name), Valid: true}
	repo.OwnerType = sql.NullString{String: strPtrOrEmpty(ghRepo.Owner.Type), Valid: true}
	repo.Private = sql.NullBool{Bool: *ghRepo.Private, Valid: true}
	repo.URL = sql.NullString{String: strPtrOrEmpty(ghRepo.Homepage), Valid: true}
}
