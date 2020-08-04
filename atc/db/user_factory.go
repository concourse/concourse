package db

import (
	"time"

	sq "github.com/Masterminds/squirrel"
)

//go:generate counterfeiter . UserFactory

type UserFactory interface {
	CreateOrUpdateUser(username, connector, sub string) error
	GetAllUsers() ([]User, error)
	GetAllUsersByLoginDate(LastLogin time.Time) ([]User, error)
}

type userFactory struct {
	conn Conn
}

func NewUserFactory(conn Conn) UserFactory {
	return &userFactory{
		conn: conn,
	}
}

func (f *userFactory) CreateOrUpdateUser(username, connector, sub string) error {
	tx, err := f.conn.Begin()

	if err != nil {
		return err
	}
	defer Rollback(tx)

	builder := psql.Insert("users").
		Columns("username", "connector", "sub").
		Values(username, connector, sub)

	_, err = builder.Suffix(`ON CONFLICT (sub) DO UPDATE SET
					username = EXCLUDED.username,
					connector = EXCLUDED.connector,
					sub = EXCLUDED.sub,
					last_login = now()`).
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (f *userFactory) GetAllUsers() ([]User, error) {
	rows, err := psql.Select("id", "username", "connector", "last_login").
		From("users").
		RunWith(f.conn).
		Query()

	if err != nil {
		return nil, err
	}

	defer Close(rows)

	var users []User

	for rows.Next() {
		var currUser user
		err = rows.Scan(&currUser.id, &currUser.name, &currUser.connector, &currUser.lastLogin)

		if err != nil {
			return nil, err
		}

		users = append(users, currUser)
	}
	return users, nil
}

func (f *userFactory) GetAllUsersByLoginDate(lastLogin time.Time) ([]User, error) {
	rows, err := psql.Select("id", "username", "connector", "last_login").
		From("users").
		Where(sq.GtOrEq{"last_login": lastLogin}).
		RunWith(f.conn).
		Query()

	if err != nil {
		return nil, err
	}

	defer Close(rows)

	var users []User

	for rows.Next() {
		var currUser user
		err = rows.Scan(&currUser.id, &currUser.name, &currUser.connector, &currUser.lastLogin)

		if err != nil {
			return nil, err
		}

		users = append(users, currUser)
	}
	return users, nil
}
