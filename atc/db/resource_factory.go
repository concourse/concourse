package db

import (
	"database/sql"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc/db/lock"
)

//go:generate counterfeiter . ResourceFactory

type ResourceFactory interface {
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

func (r *resourceFactory) VisibleResources(teamNames []string) ([]Resource, error) {
	rows, err := resourcesQuery.
		Where(
			sq.Or{
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

	return r.parseResources(rows)
}

func (r *resourceFactory) AllResources() ([]Resource, error) {
	rows, err := resourcesQuery.
		OrderBy("r.id ASC").
		RunWith(r.conn).
		Query()
	if err != nil {
		return nil, err
	}

	return r.parseResources(rows)
}

func (r *resourceFactory) parseResources(resourceRows *sql.Rows) ([]Resource, error) {
	var resources []Resource

	for resourceRows.Next() {
		resource := &resource{conn: r.conn, lockFactory: r.lockFactory}

		err := scanResource(resource, resourceRows)
		if err != nil {
			return nil, err
		}

		resources = append(resources, resource)
	}

	return resources, nil
}
