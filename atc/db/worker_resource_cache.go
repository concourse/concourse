package db

import (
	"database/sql"
	"errors"

	sq "github.com/Masterminds/squirrel"
)

type WorkerResourceCache struct {
	WorkerName       string
	ResourceCache    UsedResourceCache
	SourceWorkerName string // If source worker doesn't equal to worker, then this is a streamed resource cache.
}

type UsedWorkerResourceCache struct {
	ID int
}

var ErrWorkerBaseResourceTypeDisappeared = errors.New("worker base resource type disappeared")

func (workerResourceCache WorkerResourceCache) FindOrCreate(tx Tx) (*UsedWorkerResourceCache, error) {
	baseResourceType := workerResourceCache.ResourceCache.BaseResourceType()
	usedWorkerBaseResourceType, found, err := WorkerBaseResourceType{
		Name:       baseResourceType.Name,
		WorkerName: workerResourceCache.SourceWorkerName,
	}.Find(tx)
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, ErrWorkerBaseResourceTypeDisappeared
	}

	id, _, found, err := workerResourceCache.find(tx, workerResourceCache.WorkerName)
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
			"worker_name",
		).
		Values(
			workerResourceCache.ResourceCache.ID(),
			usedWorkerBaseResourceType.ID,
			workerResourceCache.WorkerName,
		).
		Suffix(`
			ON CONFLICT (resource_cache_id, worker_base_resource_type_id, worker_name) DO UPDATE SET
				resource_cache_id = ?,
				worker_base_resource_type_id = ?,
				worker_name = ? 
			RETURNING id
		`, workerResourceCache.ResourceCache.ID(), usedWorkerBaseResourceType.ID, workerResourceCache.WorkerName).
		RunWith(tx).
		QueryRow().
		Scan(&id)
	if err != nil {
		return nil, err
	}

	return &UsedWorkerResourceCache{
		ID: id,
	}, nil
}

func (workerResourceCache WorkerResourceCache) Find(runner sq.Runner) (*UsedWorkerResourceCache, bool, error) {
	id, workerBasedResourceTypeId, found, err := workerResourceCache.find(runner, workerResourceCache.WorkerName)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	// Verify worker base resource type
	_, found, err = WorkerBaseResourceType{}.FindById(runner, workerBasedResourceTypeId)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	return &UsedWorkerResourceCache{ID: id}, true, nil
}

func (workerResourceCache WorkerResourceCache) find(runner sq.Runner, workerName string) (int, int, bool, error) {
	var id, workerBasedResourceTypeId int

	err := psql.Select("id, worker_base_resource_type_id").
		From("worker_resource_caches").
		Where(sq.Eq{
			"resource_cache_id": workerResourceCache.ResourceCache.ID(),
			"worker_name":       workerName,
		}).
		Suffix("FOR SHARE").
		RunWith(runner).
		QueryRow().
		Scan(&id, &workerBasedResourceTypeId)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, 0, false, nil
		}

		return 0, 0, false, err
	}

	return id, workerBasedResourceTypeId, true, nil
}
