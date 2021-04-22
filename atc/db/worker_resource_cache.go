package db

import (
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
	if workerResourceCache.SourceWorkerName == "" {
		workerResourceCache.SourceWorkerName = workerResourceCache.WorkerName
	}

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

	ids, workerBaseResourceTypeIds, found, err := workerResourceCache.find(tx, workerResourceCache.WorkerName)
	if err != nil {
		return nil, err
	}

	if found {
		for i, workerBaseResourceTypeId := range workerBaseResourceTypeIds {
			if workerBaseResourceTypeId == usedWorkerBaseResourceType.ID {
				return &UsedWorkerResourceCache{
					ID: ids[i],
				}, nil
			}
		}
	}

	var id int
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

// Find looks for a worker resource cache by resource cache id and worker name. It may find multiple
// worker resource caches, just return the first valid one.
// Note: Find doesn't consider workerResourceCache.SourceWorker as a search condition.
func (workerResourceCache WorkerResourceCache) Find(runner sq.Runner) (*UsedWorkerResourceCache, bool, error) {
	ids, workerBasedResourceTypeIds, found, err := workerResourceCache.find(runner, workerResourceCache.WorkerName)
	if err != nil {
		return nil, false, err
	}

	if found {
		// Verify worker base resource type
		for i, workerBasedResourceTypeId := range workerBasedResourceTypeIds {
			_, foundBrt, err := WorkerBaseResourceType{}.FindById(runner, workerBasedResourceTypeId)
			if err != nil {
				return nil, false, err
			}
			if foundBrt {
				return &UsedWorkerResourceCache{ID: ids[i]}, true, nil
			}
		}
	}

	return nil, false, nil
}

func (workerResourceCache WorkerResourceCache) find(runner sq.Runner, workerName string) ([]int, []int, bool, error) {
	var ids, workerBasedResourceTypeIds []int

	rows, err := psql.Select("id, worker_base_resource_type_id").
		From("worker_resource_caches").
		Where(sq.Eq{
			"resource_cache_id": workerResourceCache.ResourceCache.ID(),
			"worker_name":       workerName,
		}).
		Suffix("FOR SHARE").
		RunWith(runner).
		Query()
	if err != nil {
		return nil, nil, false, err
	}
	defer rows.Close()

	for rows.Next() {
		var id, workerBasedResourceTypeId int
		err := rows.Scan(&id, &workerBasedResourceTypeId)
		if err != nil {
			return nil, nil, false, err
		}
		ids = append(ids, id)
		workerBasedResourceTypeIds = append(workerBasedResourceTypeIds, workerBasedResourceTypeId)
	}

	if len(workerBasedResourceTypeIds) == 0 {
		return nil, nil, false, nil
	}

	return ids, workerBasedResourceTypeIds, true, nil
}
