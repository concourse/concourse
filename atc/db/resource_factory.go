package db

import (
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc/db/lock"
)

//go:generate counterfeiter . ResourceFactory

type ResourceFactory interface {
	VisibleResources([]string) ([]Resource, error)
	GetResourcesByWebhook(string) (Resources, error)
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

	var resources []Resource

	for rows.Next() {
		resource := &resource{conn: r.conn}

		err := scanResource(resource, rows)
		if err != nil {
			return nil, err
		}

		resources = append(resources, resource)
	}

	return resources, nil
}

func (r *resourceFactory) GetResourcesByWebhook(name string) (Resources, error) {
	rows, err := resourcesQuery.
		Where("? IN (json_array_elements(r.config::json->>'{webhooks}'))", name).
		RunWith(r.conn).
		Query()
	if err != nil {
		return nil, err
	}
	defer Close(rows)

	var resources Resources

	for rows.Next() {
		newResource := &resource{conn: r.conn}
		err := scanResource(newResource, rows)
		if err != nil {
			return nil, err
		}

		resources = append(resources, newResource)
	}

	return resources, nil
}
