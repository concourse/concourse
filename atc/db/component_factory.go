package db

import (
	"database/sql"
	"time"

	sq "github.com/Masterminds/squirrel"
)

//go:generate counterfeiter . ComponentFactory

type ComponentFactory interface {
	Find(string) (Component, bool, error)
	UpdateIntervals(map[string]time.Duration) error
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

func (f *componentFactory) UpdateIntervals(components map[string]time.Duration) error {
	tx, err := f.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	for name, interval := range components {
		err := f.updateInterval(tx, name, interval)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (f *componentFactory) updateInterval(tx Tx, componentName string, interval time.Duration) error {
	result, err := psql.Update("components").
		Set("interval", interval.String()).
		Where(sq.Eq{
			"name": componentName,
		}).
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
		return nonOneRowAffectedError{rowsAffected}
	}

	return nil
}
