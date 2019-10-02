package db

import (
	"database/sql"

	sq "github.com/Masterminds/squirrel"
)

//go:generate counterfeiter . ComponentFactory

type ComponentFactory interface {
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
