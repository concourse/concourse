package db

import (
	"errors"
	sq "github.com/Masterminds/squirrel"
	"math/rand"
	"time"
)

type WorkerResourceCache struct {
	WorkerName    string
	ResourceCache UsedResourceCache
}

type UsedWorkerResourceCache struct {
	ID int
}

var ErrWorkerBaseResourceTypeDisappeared = errors.New("worker base resource type disappeared")

func (workerResourceCache WorkerResourceCache) FindOrCreate(tx Tx, sourceWorker string) (*UsedWorkerResourceCache, error) {
	if sourceWorker == "" {
		sourceWorker = workerResourceCache.WorkerName
	}

	baseResourceType := workerResourceCache.ResourceCache.BaseResourceType()
	usedWorkerBaseResourceType, found, err := WorkerBaseResourceType{
		Name:       baseResourceType.Name,
		WorkerName: sourceWorker,
	}.Find(tx)
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, ErrWorkerBaseResourceTypeDisappeared
	}

	ids, workerBaseResourceTypeIds, found, err := workerResourceCache.find(tx)
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
	ids, _, found, err := workerResourceCache.find(runner)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	// If there are multiple workers found, choose a random one.
	index := 0
	if len(ids) > 1 {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		index = r.Intn(len(ids))
	}
	return &UsedWorkerResourceCache{ID: ids[index]}, true, nil
}

func (workerResourceCache WorkerResourceCache) find(runner sq.Runner) ([]int, []int, bool, error) {
	var ids, workerBasedResourceTypeIds []int

	rows, err := psql.Select("id, worker_base_resource_type_id").
		From("worker_resource_caches").
		Where(sq.Eq{
			"resource_cache_id": workerResourceCache.ResourceCache.ID(),
			"worker_name":       workerResourceCache.WorkerName,
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
