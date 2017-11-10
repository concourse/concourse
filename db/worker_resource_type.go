package db

import (
	"database/sql"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/lib/pq"
)

type WorkerBaseResourceTypeAlreadyExistsError struct {
	WorkerName           string
	BaseResourceTypeName string
}

func (e WorkerBaseResourceTypeAlreadyExistsError) Error() string {
	return fmt.Sprintf("worker '%s' base resource type '%s' already exists", e.WorkerName, e.BaseResourceTypeName)
}

// base_resource_types: <- gced referenced by 0 workers
// | id | type | image | version |

// worker_resource_types: <- synced w/ worker creation
// | worker_name | base_resource_type_id |

// resource_caches: <- gced by cache collector
// | id | resource_cache_id | base_resource_type_id | source_hash | params_hash | version |

type WorkerResourceType struct {
	Worker  Worker
	Image   string // The path to the image, e.g. '/opt/concourse/resources/git'.
	Version string // The version of the image, e.g. a SHA of the rootfs.

	BaseResourceType *BaseResourceType
}

type UsedWorkerResourceType struct {
	ID int

	Worker Worker

	UsedBaseResourceType *UsedBaseResourceType
}

func (wrt WorkerResourceType) FindOrCreate(tx Tx) (*UsedWorkerResourceType, error) {
	usedBaseResourceType, err := wrt.BaseResourceType.FindOrCreate(tx)
	if err != nil {
		return nil, err
	}
	uwrt, found, err := wrt.find(tx, usedBaseResourceType)
	if err != nil {
		return nil, err
	}

	if found {
		return uwrt, nil
	}

	return wrt.create(tx, usedBaseResourceType)
}

func (wrt WorkerResourceType) find(tx Tx, usedBaseResourceType *UsedBaseResourceType) (*UsedWorkerResourceType, bool, error) {
	var (
		workerName string
		id         int
	)
	err := psql.Select("id", "worker_name").From("worker_base_resource_types").Where(sq.Eq{
		"worker_name":           wrt.Worker.Name(),
		"base_resource_type_id": usedBaseResourceType.ID,
		"image":                 wrt.Image,
		"version":               wrt.Version,
	}).RunWith(tx).QueryRow().Scan(&id, &workerName)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	return &UsedWorkerResourceType{
		ID:                   id,
		Worker:               wrt.Worker,
		UsedBaseResourceType: usedBaseResourceType,
	}, true, nil
}

func (wrt WorkerResourceType) create(tx Tx, usedBaseResourceType *UsedBaseResourceType) (*UsedWorkerResourceType, error) {
	var id int
	err := psql.Insert("worker_base_resource_types").
		Columns(
			"worker_name",
			"base_resource_type_id",
			"image",
			"version",
		).
		Values(
			wrt.Worker.Name(),
			usedBaseResourceType.ID,
			wrt.Image,
			wrt.Version,
		).
		Suffix("RETURNING id").
		RunWith(tx).
		QueryRow().
		Scan(&id)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == pqUniqueViolationErrCode {
			return nil, WorkerBaseResourceTypeAlreadyExistsError{
				WorkerName:           wrt.Worker.Name(),
				BaseResourceTypeName: usedBaseResourceType.Name,
			}
		}

		return nil, err
	}

	return &UsedWorkerResourceType{
		ID:                   id,
		Worker:               wrt.Worker,
		UsedBaseResourceType: usedBaseResourceType,
	}, nil
}
