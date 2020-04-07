package db

import (
	"database/sql"
	"time"

	sq "github.com/Masterminds/squirrel"
)

type user struct {
	id        int
	sub       string
	name      string
	connector string
	lastLogin time.Time
}

//go:generate counterfeiter . User

type User interface {
	ID() int
	Sub() string
	Name() string
	Connector() string
	LastLogin() time.Time
}

func (u user) ID() int              { return u.id }
func (u user) Sub() string          { return u.sub }
func (u user) Name() string         { return u.name }
func (u user) Connector() string    { return u.connector }
func (u user) LastLogin() time.Time { return u.lastLogin }

func (u user) find(tx Tx) (User, bool, error) {
	var (
		id        int
		lastLogin time.Time
	)

	err := psql.Select("id", "last_login").
		From("users").
		Where(sq.Eq{
			"username":  u.name,
			"connector": u.connector,
			"sub":       u.sub,
		}).
		RunWith(tx).
		QueryRow().
		Scan(&id, &lastLogin)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
	}
	return &user{
		id:        id,
		sub:       u.sub,
		name:      u.name,
		connector: u.connector,
		lastLogin: lastLogin,
	}, true, nil
}

func (u user) create(tx Tx) (User, error) {
	var (
		id        int
		lastLogin time.Time
	)

	err := psql.Insert("users").
		Columns("username", "connector", "sub").
		Values(u.name, u.connector, u.sub).
		Suffix(`ON CONFLICT (sub) DO UPDATE SET
				username = ?,
				connector = ?,
				sub = ?,
				last_login = now() 
			RETURNING id, last_login`, u.name, u.connector, u.sub).
		RunWith(tx).
		QueryRow().
		Scan(&id, &lastLogin)
	if err != nil {
		return nil, err
	}

	return &user{id: id, name: u.name, connector: u.connector, lastLogin: lastLogin, sub: u.sub}, nil
}
