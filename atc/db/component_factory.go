package db

import (
	"database/sql"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
)

//go:generate counterfeiter . ComponentFactory

type ComponentFactory interface {
	CreateOrUpdate(atc.Component) (Component, error)
	Find(string) (Component, bool, error)
}

type componentFactory struct {
	conn Conn
}

func NewComponentFactory(conn Conn) ComponentFactory {
	return &componentFactory{
		conn: conn,
	}
}

func (f *componentFactory) Find(componentName string) (Component, bool, error) {
	component := &component{
		conn: f.conn,
	}

	row := componentsQuery.
		Where(sq.Eq{"c.name": componentName}).
		RunWith(f.conn).
		QueryRow()

	err := scanComponent(component, row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	return component, true, nil
}

func (f *componentFactory) CreateOrUpdate(c atc.Component) (Component, error) {
	tx, err := f.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	obj := &component{
		conn: f.conn,
	}

	row := psql.Insert("components").
		Columns("name", "interval").
		Values(c.Name, c.Interval.String()).
		Suffix(`
			ON CONFLICT (name) DO UPDATE SET interval=EXCLUDED.interval
			RETURNING id, name, interval, last_ran, paused
		`).
		RunWith(tx).
		QueryRow()
	if err != nil {
		return nil, err
	}

	err = scanComponent(obj, row)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return obj, nil
}
