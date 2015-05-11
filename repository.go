package accountsync

import (
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx/types"
)

type Repository struct {
	ID                  int            `db:"id"`
	Name                sql.NullString `db:"name"`
	URL                 sql.NullString `db:"url"`
	CreatedAt           *time.Time     `db:"created_at"`
	UpdatedAt           *time.Time     `db:"updated_at"`
	LastBuildID         int            `db:"last_build_id"`
	LastBuildNumber     sql.NullString `db:last_build_number"`
	LastbuildStartedAt  *time.Time     `db:"last_build_started_at"`
	LastBuildFinishedAt *time.Time     `db:"last_build_finished_at"`
	OwnerName           sql.NullString `db:"owner_name"`
	OwnerEmail          sql.NullString `db:"owner_email"`
	Active              bool           `db:"active"`
	Description         sql.NullString `db:"description"`
	LastBuildDuration   int            `db:"last_build_duration"`
	OwnerID             int            `db:"owner_id"`
	OwnerType           sql.NullString `db:"owner_type"`
	Private             bool           `db:"private"`
	LastBuildState      sql.NullString `db:"last_build_state"`
	GithubID            int            `db:"github_id"`
	DefaultBranch       sql.NullString `db:"default_branch"`
	GithubLanguage      sql.NullString `db:"github_language"`
	Settings            types.JsonText `db:"settings"`
	NextBuildNumber     int            `db:"next_build_number"`
}
