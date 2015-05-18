package accountsync

import (
	"database/sql"
	"time"
)

type Organization struct {
	ID sql.NullInt64 `db:"id"`

	Name      sql.NullString `db:"name"`
	Login     sql.NullString `db:"login"`
	GithubID  sql.NullInt64  `db:"github_id"`
	CreatedAt *time.Time     `db:"created_at"`
	UpdatedAt *time.Time     `db:"updated_at"`
	AvatarURL sql.NullString `db:"avatar_url"`
	Location  sql.NullString `db:"location"`
	Email     sql.NullString `db:"email"`
	Company   sql.NullString `db:"company"`
	Homepage  sql.NullString `db:"homepage"`
}
