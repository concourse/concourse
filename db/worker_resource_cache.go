package db

import (
	"database/sql"
	"errors"

	sq "github.com/Masterminds/squirrel"
	"github.com/lib/pq"
)

type WorkerResourceCache struct {
	WorkerName    string
	ResourceCache *UsedResourceCache
}

type UsedWorkerResourceCache struct {
	ID int
}

var ErrWorkerBaseResourceTypeDisappeared = errors.New("worker base resource type disappeared")

func (workerResourceCache WorkerResourceCache) FindOrCreate(tx Tx) (*UsedWorkerResourceCache, error) {
	baseResourceType := workerResourceCache.ResourceCache.BaseResourceType()
	usedWorkerBaseResourceType, found, err := WorkerBaseResourceType{
		Name:       baseResourceType.Name,
		WorkerName: workerResourceCache.WorkerName,
	}.Find(tx)
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, ErrWorkerBaseResourceTypeDisappeared
	}

	id, found, err := workerResourceCache.find(tx, usedWorkerBaseResourceType)
	if err != nil {
		return nil, err
	}

	if found {
		return &UsedWorkerResourceCache{
			ID: id,
		}, nil
	}

	err = psql.Insert("worker_resource_caches").
		Columns(
			"resource_cache_id",
			"worker_base_resource_type_id",
		).
		Values(
			workerResourceCache.ResourceCache.ID,
			usedWorkerBaseResourceType.ID,
		).
		Suffix("RETURNING id").
		RunWith(tx).
		QueryRow().
		Scan(&id)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == pqUniqueViolationErrCode {
			return nil, ErrSafeRetryFindOrCreate
		}

		return nil, err
	}

	return &UsedWorkerResourceCache{
		ID: id,
	}, nil
}

func (workerResourceCache WorkerResourceCache) Find(runner sq.Runner) (*UsedWorkerResourceCache, bool, error) {
	baseResourceType := workerResourceCache.ResourceCache.BaseResourceType()
	usedWorkerBaseResourceType, found, err := WorkerBaseResourceType{
		Name:       baseResourceType.Name,
		WorkerName: workerResourceCache.WorkerName,
	}.Find(runner)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	id, found, err := workerResourceCache.find(runner, usedWorkerBaseResourceType)
	if err != nil {
		return nil, false, err
	}

	if found {
		return &UsedWorkerResourceCache{
			ID: id,
		}, true, nil
	}

	return nil, false, nil
}

func (workerResourceCache WorkerResourceCache) find(runner sq.Runner, usedWorkerBaseResourceType *UsedWorkerBaseResourceType) (int, bool, error) {
	var id int

	err := psql.Select("id").
		From("worker_resource_caches").
		Where(sq.Eq{
			"resource_cache_id":            workerResourceCache.ResourceCache.ID,
			"worker_base_resource_type_id": usedWorkerBaseResourceType.ID,
		}).
		RunWith(runner).
		QueryRow().
		Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, false, nil
		}

		return 0, false, err
	}

	return id, true, nil
}
