package db

import (
	"database/sql"
	sq "github.com/Masterminds/squirrel"
	"time"
)

//go:generate counterfeiter . Wall

type Wall interface {
	SetMessage(string) error
	GetMessage() (string, error)
	SetExpiration(time.Duration) error
	GetExpiration() (time.Time, error)
	Clear() error
}

type wall struct {
	conn Conn
}

func NewWall(conn Conn) Wall {
	return &wall{conn: conn}
}

func (w wall) SetMessage(message string) error {
	err := w.Clear()
	if err != nil {
		return err
	}

	_, err = psql.Insert("wall").
		Columns("message").
		Values(message).
		RunWith(w.conn).Exec()
	if err != nil {
		return err
	}

	return nil
}

func (w wall) GetMessage() (string, error) {
	var message string
	err := psql.Select("message").
		From("wall").
		Where(sq.Or{
			sq.Gt{"expires_at": time.Now()},
			sq.Eq{"expires_at": nil},
		}).
		RunWith(w.conn).QueryRow().Scan(&message)
	if err != nil && err != sql.ErrNoRows {
		return "", err
	}

	return message, nil
}

func (w wall) SetExpiration(duration time.Duration) error {
	expiration := time.Now().Add(duration)
	_, err := psql.Update("wall").
		Set("expires_at", expiration).
		RunWith(w.conn).Exec()
	if err != nil {
		return err
	}
	return nil
}

func (w wall) GetExpiration() (time.Time, error) {
	var expiration time.Time
	err := psql.Select("expires_at").
		From("wall").
		RunWith(w.conn).QueryRow().Scan(&expiration)
	if err != nil {
		if err == sql.ErrNoRows {
			return expiration, nil
		}
		return expiration, err
	}
	return expiration, nil
}

func (w wall) Clear() error {
	_, err := psql.Delete("wall").RunWith(w.conn).Exec()
	if err != nil {
		return err
	}
	return nil
}
