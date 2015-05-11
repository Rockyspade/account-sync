package accountsync

import (
	"database/sql"
	"time"

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
	LastBuildNumber     sql.NullString `db:last_build_number"`
	LastBuildState      sql.NullString `db:"last_build_state"`
	LastbuildStartedAt  *time.Time     `db:"last_build_started_at"`
	Name                sql.NullString `db:"name"`
	NextBuildNumber     int64          `db:"next_build_number"`
	OwnerEmail          sql.NullString `db:"owner_email"`
	OwnerID             int64          `db:"owner_id"`
	OwnerName           sql.NullString `db:"owner_name"`
	OwnerType           sql.NullString `db:"owner_type"`
	Private             bool           `db:"private"`
	Settings            types.JsonText `db:"settings"`
	URL                 sql.NullString `db:"url"`
	UpdatedAt           *time.Time     `db:"updated_at"`
}
