package accountsync

import (
	"database/sql"
	"time"

	"github.com/google/go-github/github"
	"github.com/jmoiron/sqlx/types"
)

type Repository struct {
	ID int64 `db:"id"`

	Active              bool           `db:"active"`
	CreatedAt           *time.Time     `db:"created_at"`
	DefaultBranch       sql.NullString `db:"default_branch"`
	Description         sql.NullString `db:"description"`
	GithubID            int64          `db:"github_id"`
	GithubLanguage      sql.NullString `db:"github_language"`
	LastBuildDuration   int64          `db:"last_build_duration"`
	LastBuildFinishedAt *time.Time     `db:"last_build_finished_at"`
	LastBuildID         int64          `db:"last_build_id"`
	LastBuildNumber     sql.NullString `db:"last_build_number"`
	LastBuildState      sql.NullString `db:"last_build_state"`
	LastBuildStartedAt  *time.Time     `db:"last_build_started_at"`
	LastSync            *time.Time     `db:"last_sync"`
	Name                sql.NullString `db:"name"`
	NextBuildNumber     sql.NullString `db:"next_build_number"`
	OwnerEmail          sql.NullString `db:"owner_email"`
	OwnerID             int64          `db:"owner_id"`
	OwnerName           sql.NullString `db:"owner_name"`
	OwnerType           sql.NullString `db:"owner_type"`
	Private             bool           `db:"private"`
	Settings            types.JsonText `db:"settings"`
	URL                 sql.NullString `db:"url"`
	UpdatedAt           *time.Time     `db:"updated_at"`
}

func (repo *Repository) UpdateFromGithubRepository(ghRepo *github.Repository) {
	repo.DefaultBranch = sql.NullString{String: *ghRepo.DefaultBranch, Valid: true}
	repo.Description = sql.NullString{String: *ghRepo.Description, Valid: true}
	repo.GithubID = int64(*ghRepo.ID)
	repo.GithubLanguage = sql.NullString{String: *ghRepo.Language, Valid: true}
	repo.Name = sql.NullString{String: *ghRepo.Name, Valid: true}
	repo.OwnerID = int64(*ghRepo.Owner.ID)
	repo.OwnerName = sql.NullString{String: *ghRepo.Owner.Name, Valid: true}
	repo.OwnerType = sql.NullString{String: *ghRepo.Owner.Type, Valid: true}
	repo.Private = *ghRepo.Private
	repo.URL = sql.NullString{String: *ghRepo.Homepage, Valid: true}
}
