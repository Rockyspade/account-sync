package accountsync

import (
	"fmt"
	"log"

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

type UserInfoSyncer struct {
	db  *sqlx.DB
	cfg *Config
}

func NewUserInfoSyncer(db *sqlx.DB, cfg *Config) *UserInfoSyncer {
	return &UserInfoSyncer{db: db, cfg: cfg}
}

func (uis *UserInfoSyncer) Sync(user *User, client *github.Client) error {
	ghUser, _, err := client.Users.Get(user.Login.String)
	if err != nil {
		return err
	}

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

	email, err := uis.getUserEmail(user, ghUser, client)
	if err != nil {
		return err
	}

	edu, err := uis.getIsEducation(user, ghUser, client)
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
		SET name = $1, login = $2, gravatar_id = $3, email = $4, education = $5
		WHERE id = $6
	`, *ghUser.Name, *ghUser.Login, *ghUser.GravatarID, email, isEdu,
		user.ID)

	if err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (uis *UserInfoSyncer) getUserEmail(user *User, ghUser *github.User, client *github.Client) (string, error) {
	email := *ghUser.Email
	if email != "" {
		return email, nil
	}

	allEmails, err := uis.getAllEmails(user, client)
	if err != nil {
		return "", err
	}

	primaryEmail := ""
	firstEmail := ""
	verifiedEmail := ""
	for i, email := range allEmails {
		if i == 0 {
			firstEmail = *email.Email
		}

		if verifiedEmail == "" && email.Verified != nil && *email.Verified {
			verifiedEmail = *email.Email
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

	if user.Email.String != "" {
		return user.Email.String, nil
	}

	return firstEmail, nil
}

func (uis *UserInfoSyncer) getAllEmails(user *User, client *github.Client) ([]github.UserEmail, error) {
	if !sliceContains(user.GithubScopes, "user") && !sliceContains(user.GithubScopes, "user:email") {
		return []github.UserEmail{}, nil
	}

	emails, _, err := client.Users.ListEmails(&github.ListOptions{
		Page:    1,
		PerPage: 100,
	})
	return emails, err
}

func (uis *UserInfoSyncer) getIsEducation(user *User, ghUser *github.User, client *github.Client) (*bool, error) {
	req, err := client.NewRequest("GET", "https://education.github.com/api/user", nil)
	if err != nil {
		return nil, err
	}

	body := map[string]bool{"student": false}

	_, err = client.Do(req, &body)
	if err != nil {
		return nil, err
	}

	student := body["student"]

	return &student, nil
}
