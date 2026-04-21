package db

import (
	"database/sql"

	"code.cloudfoundry.org/clock"
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
)

//counterfeiter:generate . ComponentFactory
type ComponentFactory interface {
	CreateOrUpdate(atc.Component) (Component, error)
	Find(string) (Component, bool, error)

	// Returns all components
	All() ([]Component, error)

	// Pauses all components
	PauseAll() error

	// Unpauses all components
	UnpauseAll() error
}

var _ ComponentFactory = (*componentFactory)(nil)

type componentFactory struct {
	conn                  DbConn
	numGoroutineThreshold int
	rander                ComponentRand
	clock                 clock.Clock
	goRoutineCounter      GoroutineCounter
}

func NewComponentFactory(conn DbConn, numGoroutineThreshold int, rander ComponentRand, clock clock.Clock, goRoutineCounter GoroutineCounter) ComponentFactory {
	return &componentFactory{
		conn:                  conn,
		numGoroutineThreshold: numGoroutineThreshold,
		rander:                rander,
		clock:                 clock,
		goRoutineCounter:      goRoutineCounter,
	}
}

func (f *componentFactory) Find(componentName string) (Component, bool, error) {
	component := &component{
		conn:                  f.conn,
		numGoroutineThreshold: f.numGoroutineThreshold,
		rander:                f.rander,
		clock:                 f.clock,
		goRoutineCounter:      f.goRoutineCounter,
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
		conn:                  f.conn,
		numGoroutineThreshold: f.numGoroutineThreshold,
		rander:                f.rander,
		clock:                 f.clock,
		goRoutineCounter:      f.goRoutineCounter,
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

func (f *componentFactory) All() ([]Component, error) {
	rows, err := componentsQuery.
		RunWith(f.conn).
		Query()
	if err != nil {
		return nil, err
	}
	defer Close(rows)

	var components []Component
	for rows.Next() {
		c := &component{
			conn:                  f.conn,
			numGoroutineThreshold: f.numGoroutineThreshold,
			rander:                f.rander,
			clock:                 f.clock,
			goRoutineCounter:      f.goRoutineCounter,
		}
		err = scanComponent(c, rows)
		if err != nil {
			return nil, err
		}
		components = append(components, c)
	}

	return components, nil
}

func (f *componentFactory) PauseAll() error {
	_, err := psql.Update("components").
		Set("paused", true).
		RunWith(f.conn).
		Exec()
	return err
}

func (f *componentFactory) UnpauseAll() error {
	_, err := psql.Update("components").
		Set("paused", false).
		RunWith(f.conn).
		Exec()
	return err
}
