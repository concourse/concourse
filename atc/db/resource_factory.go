package db

import (
	"database/sql"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc/db/lock"
)

//go:generate counterfeiter . ResourceFactory

type ResourceFactory interface {
	Resource(int) (Resource, bool, error)
	VisibleResources([]string) ([]Resource, error)
	AllResources() ([]Resource, error)
}

type resourceFactory struct {
	conn        Conn
	lockFactory lock.LockFactory
}

func NewResourceFactory(conn Conn, lockFactory lock.LockFactory) ResourceFactory {
	return &resourceFactory{
		conn:        conn,
		lockFactory: lockFactory,
	}
}

func (r *resourceFactory) Resource(resourceID int) (Resource, bool, error) {
	resource := newEmptyResource(r.conn, r.lockFactory)
	row := resourcesQuery.
		Where(sq.Eq{"r.id": resourceID}).
		RunWith(r.conn).
		QueryRow()

	err := scanResource(resource, row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	return resource, true, nil
}

func (r *resourceFactory) VisibleResources(teamNames []string) ([]Resource, error) {
	rows, err := resourcesQuery.
		Where(sq.Or{
			sq.Eq{"t.name": teamNames},
			sq.And{
				sq.NotEq{"t.name": teamNames},
				sq.Eq{"p.public": true},
			},
		}).
		OrderBy("r.id ASC").
		RunWith(r.conn).
		Query()
	if err != nil {
		return nil, err
	}

	return scanResources(rows, r.conn, r.lockFactory)
}

func (r *resourceFactory) AllResources() ([]Resource, error) {
	rows, err := resourcesQuery.
		OrderBy("r.id ASC").
		RunWith(r.conn).
		Query()
	if err != nil {
		return nil, err
	}

	return scanResources(rows, r.conn, r.lockFactory)
}

func scanResources(resourceRows *sql.Rows, conn Conn, lockFactory lock.LockFactory) ([]Resource, error) {
	var resources []Resource

	for resourceRows.Next() {
		resource := newEmptyResource(conn, lockFactory)
		err := scanResource(resource, resourceRows)
		if err != nil {
			return nil, err
		}

		resources = append(resources, resource)
	}

	return resources, nil
}
