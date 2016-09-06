package dbng

import (
	"database/sql"
	"errors"

	sq "github.com/Masterminds/squirrel"
	"github.com/lib/pq"
)

type WorkerResourceType struct {
	WorkerName string
	Type       string
	Image      string
	Version    string
}

func (wrt WorkerResourceType) Lookup(tx Tx) (int, bool, error) {
	var id int
	err := psql.Select("id").From("worker_resource_types").Where(sq.Eq{
		"worker_name": wrt.WorkerName,
		"type":        wrt.Type,
		"image":       wrt.Image,
		"version":     wrt.Version,
	}).RunWith(tx).QueryRow().Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, false, nil
		}

		return 0, false, err
	}

	return id, true, nil
}

var ErrWorkerResourceTypeAlreadyExists = errors.New("worker resource type already exists")

func (wrt WorkerResourceType) Create(tx Tx) (int, error) {
	var id int
	err := psql.Insert("worker_resource_types").
		Columns(
			"worker_name",
			"type",
			"image",
			"version",
		).
		Values(
			wrt.WorkerName,
			wrt.Type,
			wrt.Image,
			wrt.Version,
		).
		Suffix("RETURNING id").
		RunWith(tx).
		QueryRow().
		Scan(&id)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "unique_violation" {
			return 0, ErrWorkerResourceTypeAlreadyExists
		}

		return 0, err
	}

	_, err = psql.Delete("worker_resource_types").
		Where(sq.And{
			sq.Eq{
				"worker_name": wrt.WorkerName,
				"type":        wrt.Type,
				"image":       wrt.Image,
			},
			sq.NotEq{
				"version": wrt.Version,
			},
		}).
		RunWith(tx).
		Exec()
	if err != nil {
		return 0, err
	}

	return id, nil
}
