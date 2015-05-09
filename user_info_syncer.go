package accountsync

import (
	"fmt"
	"log"
	"time"

	"github.com/google/go-github/github"
	"github.com/jmoiron/sqlx"
)

type UserSyncError struct {
	TravisLogin    string
	TravisGithubID int
	GithubLogin    string
	GithubID       int
}

func (err *UserSyncError) Error() string {
	return fmt.Sprintf("msg=\"updating user failed because of mismatched data\" "+
		"travis_github_id=%v github_id=%v travis_login=%v github_login=%v",
		err.TravisGithubID, err.GithubID, err.TravisLogin, err.GithubLogin)
}

type userInfoSyncContext struct {
	user           *User
	ghUser         *github.User
	client         *github.Client
	allEmails      []github.UserEmail
	verifiedEmails []string
	currentEmails  []string
}

type UserInfoSyncer struct {
	db  *sqlx.DB
	cfg *Config
}

func NewUserInfoSyncer(db *sqlx.DB, cfg *Config) *UserInfoSyncer {
	return &UserInfoSyncer{db: db, cfg: cfg}
}

func (uis *UserInfoSyncer) Sync(user *User, client *github.Client) error {
	ctx := &userInfoSyncContext{
		user:           user,
		client:         client,
		allEmails:      []github.UserEmail{},
		verifiedEmails: []string{},
		currentEmails:  []string{},
	}

	ghUser, _, err := client.Users.Get(user.Login.String)
	if err != nil {
		return err
	}

	ctx.ghUser = ghUser

	syncErr := &UserSyncError{
		TravisLogin:    user.Login.String,
		TravisGithubID: user.GithubID,
		GithubLogin:    *ghUser.Login,
		GithubID:       *ghUser.ID,
	}

	if user.GithubID != *ghUser.ID {
		return syncErr
	}

	if user.Login.String != *ghUser.Login {
		return syncErr
	}

	email, err := uis.getUserEmail(ctx)
	if err != nil {
		return err
	}

	edu, err := uis.getIsEducation(ctx)
	if err != nil {
		return err
	}

	isEdu := user.Education
	if edu != nil {
		isEdu = *edu
	}

	tx, err := uis.db.Beginx()
	if err != nil {
		return err
	}

	log.Printf("msg=\"updating user info\" sync=user_info login=%v", user.Login.String)
	_, err = tx.Exec(`
		UPDATE users
		SET name = $1, login = $2, gravatar_id = $3, email = $4, education = $5,
		    updated_at = $6
		WHERE id = $7
	`, *ghUser.Name, *ghUser.Login, *ghUser.GravatarID, email, isEdu,
		time.Now().UTC(),
		user.ID)

	if err != nil {
		tx.Rollback()
		return err
	}

	err = uis.updateEmails(tx, ctx)

	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func (uis *UserInfoSyncer) getUserEmail(ctx *userInfoSyncContext) (string, error) {
	email := *ctx.ghUser.Email
	if email != "" {
		return email, nil
	}

	allEmails, err := uis.getAllEmails(ctx)
	if err != nil {
		return "", err
	}

	ctx.allEmails = allEmails

	primaryEmail := ""
	firstEmail := ""
	verifiedEmail := ""
	for i, email := range allEmails {
		ctx.currentEmails = append(ctx.currentEmails, *email.Email)
		if i == 0 {
			firstEmail = *email.Email
		}

		if email.Verified != nil && *email.Verified {
			ctx.verifiedEmails = append(ctx.verifiedEmails, *email.Email)
			if verifiedEmail == "" {
				verifiedEmail = *email.Email
			}
		}

		if primaryEmail == "" && email.Primary != nil && *email.Primary {
			primaryEmail = *email.Email
		}
	}

	if primaryEmail != "" {
		return primaryEmail, nil
	}

	if verifiedEmail != "" {
		return verifiedEmail, nil
	}

	if ctx.user.Email.String != "" {
		return ctx.user.Email.String, nil
	}

	return firstEmail, nil
}

func (uis *UserInfoSyncer) getAllEmails(ctx *userInfoSyncContext) ([]github.UserEmail, error) {
	if !sliceContains(ctx.user.GithubScopes, "user") && !sliceContains(ctx.user.GithubScopes, "user:email") {
		return []github.UserEmail{}, nil
	}

	emails, _, err := ctx.client.Users.ListEmails(&github.ListOptions{
		Page:    1,
		PerPage: 100,
	})
	return emails, err
}

func (uis *UserInfoSyncer) getIsEducation(ctx *userInfoSyncContext) (*bool, error) {
	req, err := ctx.client.NewRequest("GET", "https://education.github.com/api/user", nil)
	if err != nil {
		return nil, err
	}

	body := map[string]bool{"student": false}

	_, err = ctx.client.Do(req, &body)
	if err != nil {
		return nil, err
	}

	student := body["student"]

	return &student, nil
}

func (uis *UserInfoSyncer) updateEmails(tx *sqlx.Tx, ctx *userInfoSyncContext) error {
	diffEmails := []string{}
	for _, email := range ctx.currentEmails {
		if !sliceContains(ctx.verifiedEmails, email) {
			diffEmails = append(diffEmails, email)
		}
	}

	if len(diffEmails) > 0 {
		query, args, err := sqlx.In(`
		DELETE FROM emails WHERE user_id = ? AND email IN (?)
	`, ctx.user.ID, diffEmails)
		if err != nil {
			return err
		}
		query = uis.db.Rebind(query)
		_, err = tx.Exec(query, args...)
		if err != nil {
			return err
		}
	}

	diffEmails = []string{}
	for _, email := range ctx.verifiedEmails {
		if !sliceContains(ctx.currentEmails, email) {
			diffEmails = append(diffEmails, email)
		}
	}

	if len(diffEmails) > 0 {
		for _, email := range diffEmails {
			now := time.Now().UTC()
			vars := map[string]interface{}{
				"user_id":    ctx.user.ID,
				"email":      email,
				"created_at": now,
				"updated_at": now,
			}
			query, args, err := sqlx.Named(`
			INSERT INTO emails VALUES (?)
		`, vars)
			query = uis.db.Rebind(query)
			_, err = tx.Exec(query, args...)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
