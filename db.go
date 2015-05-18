package accountsync

import (
	"database/sql"

	"github.com/hashicorp/golang-lru"
	"github.com/jmoiron/sqlx"
)

type DB struct {
	*sqlx.DB

	uc *lru.Cache
	oc *lru.Cache
}

func NewDB(databaseURL string, syncCacheSize int) (*DB, error) {
	db, err := sqlx.Connect("postgres", databaseURL)
	if err != nil {
		return nil, err
	}

	uc, err := lru.New(syncCacheSize)
	if err != nil {
		return nil, err
	}

	oc, err := lru.New(syncCacheSize)
	if err != nil {
		return nil, err
	}

	return &DB{
		DB: db,
		uc: uc,
		oc: oc,
	}, nil
}

func (db *DB) FindUserByGithubID(ghUserID int) (*User, error) {
	var (
		user *User
		ok   bool
	)

	u, found := db.uc.Get(ghUserID)
	if user, ok = u.(*User); found && ok {
		return user, nil
	}

	user = &User{}
	err := db.Get(user, `SELECT * FROM users WHERE github_id = $1`, ghUserID)
	if err == sql.ErrNoRows {
		user = nil
		err = nil
	}

	if user != nil {
		db.uc.Add(ghUserID, user)
	}

	return user, err
}

func (db *DB) FindOrgByGithubID(ghOrgID int) (*Organization, error) {
	var (
		org *Organization
		ok  bool
	)

	o, found := db.oc.Get(ghOrgID)
	if org, ok = o.(*Organization); found && ok {
		return org, nil
	}

	org = &Organization{}
	err := db.Get(org, `SELECT * FROM organizations WHERE github_id = $1`, ghOrgID)
	if err == sql.ErrNoRows {
		org = nil
		err = nil
	}

	if org != nil {
		db.oc.Add(ghOrgID, org)
	}

	return org, err
}
