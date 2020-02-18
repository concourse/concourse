package db

import (
	"database/sql"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
)

//go:generate counterfeiter . ComponentFactory

type ComponentFactory interface {
	Find(string) (Component, bool, error)
	UpdateIntervals([]atc.Component) error
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

func (f *componentFactory) UpdateIntervals(components []atc.Component) error {
	tx, err := f.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	for _, component := range components {
		err := f.updateInterval(tx, component)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (f *componentFactory) updateInterval(tx Tx, component atc.Component) error {
	result, err := psql.Insert("components").
		Columns("name", "interval").
		Values(component.Name, component.Interval.String()).
		Suffix("ON CONFLICT (name) DO UPDATE SET interval=EXCLUDED.interval").
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected != 1 {
		return NonOneRowAffectedError{rowsAffected}
	}

	return nil
}
