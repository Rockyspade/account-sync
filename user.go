package accountsync

import (
	"database/sql"
	"fmt"
	"time"

	"gopkg.in/yaml.v2"
)

var (
	errInvalidGithubScopesYAML = fmt.Errorf("the GithubScopesYAML value is invalid")
)

type User struct {
	ID sql.NullInt64 `db:"id"`

	CreatedAt        *time.Time     `db:"created_at"`
	Education        sql.NullBool   `db:"education"`
	Email            sql.NullString `db:"email"`
	GithubID         sql.NullInt64  `db:"github_id"`
	GithubOauthToken sql.NullString `db:"github_oauth_token"`
	GithubScopesYAML sql.NullString `db:"github_scopes"`
	GravatarID       sql.NullString `db:"gravatar_id"`
	IsAdmin          sql.NullBool   `db:"is_admin"`
	IsSyncing        sql.NullBool   `db:"is_syncing"`
	Locale           sql.NullString `db:"locale"`
	Login            sql.NullString `db:"login"`
	Name             sql.NullString `db:"name"`
	SyncedAt         *time.Time     `db:"synced_at"`
	UpdatedAt        *time.Time     `db:"updated_at"`

	GithubScopes  []string
	Organizations []*Organization
}

func (user *User) Hydrate() error {
	if user.GithubScopes != nil {
		return nil
	}

	user.GithubScopes = []string{}

	if !user.GithubScopesYAML.Valid {
		return errInvalidGithubScopesYAML
	}

	return yaml.Unmarshal([]byte(user.GithubScopesYAML.String), &user.GithubScopes)
}

func (user *User) HydrateOrganizations(db *DB) error {
	if user.Organizations != nil {
		return nil
	}

	user.Organizations = []*Organization{}
	return db.Select(&user.Organizations, `
		SELECT *
		FROM organizations
		WHERE id IN (
			SELECT organization_id
			FROM memberships
			WHERE user_id = $1
		)`, user.ID)
}
