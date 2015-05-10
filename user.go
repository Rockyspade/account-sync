package accountsync

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"gopkg.in/yaml.v2"
)

var (
	errInvalidGithubScopesYAML = fmt.Errorf("the GithubScopesYAML value is invalid")
)

type User struct {
	ID               int            `db:"id"`
	Name             sql.NullString `db:"name"`
	Login            sql.NullString `db:"login"`
	Email            sql.NullString `db:"email"`
	CreatedAt        *time.Time     `db:"created_at"`
	UpdatedAt        *time.Time     `db:"updated_at"`
	IsAdmin          bool           `db:"is_admin"`
	GithubID         int            `db:"github_id"`
	GithubOauthToken sql.NullString `db:"github_oauth_token"`
	GravatarID       sql.NullString `db:"gravatar_id"`
	IsSyncing        bool           `db:"is_syncing"`
	Locale           sql.NullString `db:"locale"`
	SyncedAt         *time.Time     `db:"synced_at"`
	GithubScopesYAML sql.NullString `db:"github_scopes"`
	Education        bool           `db:"education"`

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

func (user *User) HydrateOrganizations(db *sqlx.DB) error {
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
