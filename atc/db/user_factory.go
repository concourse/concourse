package db

import (
	sq "github.com/Masterminds/squirrel"
	"time"
)

//go:generate counterfeiter . UserFactory

type UserFactory interface {
	CreateOrUpdateUser(username, connector string) (User, error)
	GetAllUsers() ([]User, error)
	GetAllUsersByLoginDate(LastLogin time.Time) ([]User, error)
}

type userFactory struct {
	conn Conn
}

func (f *userFactory) CreateOrUpdateUser(username, connector string) (User, error) {
	tx, err := f.conn.Begin()

	if err != nil {
		return nil, err
	}
	defer Rollback(tx)

	u, err := user{
		name:      username,
		connector: connector,
	}.create(tx)

	if err != nil {
		return nil, err
	}

	err = tx.Commit()

	if err != nil {
		return nil, err
	}

	return u, nil
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

func NewUserFactory(conn Conn) UserFactory {
	return &userFactory{
		conn: conn,
	}
}
