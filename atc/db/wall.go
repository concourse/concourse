package db

import (
	"database/sql"

	"github.com/concourse/concourse/atc"

	sq "github.com/Masterminds/squirrel"
)

//go:generate counterfeiter . Wall

type Wall interface {
	SetWall(atc.Wall) error
	GetWall() (atc.Wall, error)
	Clear() error
}

type wall struct {
	conn  Conn
	clock Clock
}

func NewWall(conn Conn, clock Clock) Wall {
	return &wall{
		conn: conn,
		clock: clock,
	}
}

func (w wall) SetWall(wall atc.Wall) error {
	tx, err := w.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	_, err = psql.Delete("wall").RunWith(tx).Exec()
	if err != nil {
		return err
	}

	query := psql.Insert("wall").
		Columns("message")

	if wall.TTL != 0 {
		expiresAt := w.clock.Now().Add(wall.TTL)
		query = query.Columns("expires_at").Values(wall.Message, expiresAt)
	} else {
		query = query.Values(wall.Message)
	}

	_, err = query.RunWith(tx).Exec()
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (w wall) GetWall() (atc.Wall, error) {
	var wall atc.Wall

	row := psql.Select("message", "expires_at").
		From("wall").
		Where(sq.Or{
			sq.Gt{"expires_at": w.clock.Now()},
			sq.Eq{"expires_at": nil},
		}).
		RunWith(w.conn).QueryRow()

	err := w.scanWall(&wall, row)
	if err != nil && err != sql.ErrNoRows {
		return atc.Wall{}, err
	}

	return wall, nil
}

func (w *wall) scanWall(wall *atc.Wall, scan scannable) error {
	var expiresAt sql.NullTime

	err := scan.Scan(&wall.Message, &expiresAt)
	if err != nil {
		return err
	}

	if expiresAt.Valid {
		wall.TTL = w.clock.Until(expiresAt.Time)
	}

	return nil
}

func (w wall) Clear() error {
	_, err := psql.Delete("wall").RunWith(w.conn).Exec()
	if err != nil {
		return err
	}
	return nil
}
