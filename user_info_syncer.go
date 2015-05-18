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
	TravisGithubID int64
	GithubLogin    string
	GithubID       int64
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
		TravisGithubID: int64(user.GithubID.Int64),
		GithubLogin:    *ghUser.Login,
		GithubID:       int64(*ghUser.ID),
	}

	if user.GithubID.Int64 != int64(*ghUser.ID) {
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

	isEdu := user.Education.Bool
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

	log.Printf("msg=\"updating emails\" sync=user_info login=%v", user.Login.String)
	err = uis.updateEmails(tx, ctx)

	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func (uis *UserInfoSyncer) getUserEmail(ctx *userInfoSyncContext) (string, error) {
	log.Printf("msg=\"fetching all emails\" level=debug sync=user_info login=%s",
		ctx.user.Login.String)

	allEmails, err := uis.getAllGithubEmails(ctx)
	if err != nil {
		return *ctx.ghUser.Email, err
	}

	ctx.allEmails = allEmails

	currentEmails, err := uis.getCurrentlySyncedEmails(ctx)
	if err != nil {
		return *ctx.ghUser.Email, err
	}

	ctx.currentEmails = currentEmails

	primaryEmail := ""
	firstEmail := ""
	verifiedEmail := ""
	for i, email := range allEmails {
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

	log.Printf("level=debug sync=user_info login=%s current_emails=%v",
		ctx.user.Login.String, ctx.currentEmails)

	log.Printf("level=debug sync=user_info login=%s verified_emails=%v",
		ctx.user.Login.String, ctx.verifiedEmails)

	email := *ctx.ghUser.Email
	if email != "" {
		return email, nil
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

func (uis *UserInfoSyncer) getAllGithubEmails(ctx *userInfoSyncContext) ([]github.UserEmail, error) {
	if !sliceContains(ctx.user.GithubScopes, "user") && !sliceContains(ctx.user.GithubScopes, "user:email") {
		return []github.UserEmail{}, nil
	}

	emails, _, err := ctx.client.Users.ListEmails(&github.ListOptions{
		Page:    1,
		PerPage: 100,
	})
	return emails, err
}

func (uis *UserInfoSyncer) getCurrentlySyncedEmails(ctx *userInfoSyncContext) ([]string, error) {
	emails := []string{}
	err := uis.db.Select(&emails, `SELECT email	FROM emails WHERE user_id = $1`, ctx.user.ID)
	if err != nil {
		return nil, err
	}

	return emails, nil
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
		log.Printf("level=debug sync=user_info login=%s query=%q args=%v",
			ctx.user.Login.String, query, args)
		_, err = tx.Exec(query, args...)
		if err != nil {
			return err
		}
	} else {
		log.Printf("msg=\"no emails to delete\" sync=user_info login=%s", ctx.user.Login.String)
	}

	diffEmails = []string{}
	for _, email := range ctx.verifiedEmails {
		if !sliceContains(ctx.currentEmails, email) {
			diffEmails = append(diffEmails, email)
		}
	}

	if len(diffEmails) == 0 {
		log.Printf("msg=\"no emails to add\" sync=user_info login=%s", ctx.user.Login.String)
		return nil
	}

	for _, email := range diffEmails {
		now := time.Now().UTC()
		vars := map[string]interface{}{
			"user_id":    ctx.user.ID,
			"email":      email,
			"created_at": now,
			"updated_at": now,
		}
		_, err := tx.NamedExec(`
			INSERT INTO emails (user_id, email, created_at, updated_at)
			VALUES (:user_id, :email, :created_at, :updated_at)`, vars)
		if err != nil {
			return err
		}
	}

	return nil
}
