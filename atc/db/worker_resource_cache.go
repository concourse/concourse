package db

import (
	"errors"
	"math/rand"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/lib/pq"
)

type WorkerResourceCache struct {
	WorkerName    string
	ResourceCache UsedResourceCache
}

type UsedWorkerResourceCache struct {
	ID int
}

var ErrWorkerBaseResourceTypeDisappeared = errors.New("worker base resource type disappeared")

// FindOrCreate finds or creates a worker_resource_cache initialized from a
// given sourceWorker. If there already exists a worker_resource_cache for the
// provided WorkerName and ResourceCache, but initialized from a different
// sourceWorker, it will return `false` as its second return value.
//
// This can happen if multiple volumes for the same resource cache are being
// streamed to a worker simultaneously from multiple other "source" workers -
// we only want a single worker_resource_cache in the end for the destination
// worker, so the "first write wins".
func (workerResourceCache WorkerResourceCache) FindOrCreate(tx Tx, sourceWorker string) (*UsedWorkerResourceCache, bool, error) {
	if sourceWorker == "" {
		sourceWorker = workerResourceCache.WorkerName
	}

	baseResourceType := workerResourceCache.ResourceCache.BaseResourceType()
	usedWorkerBaseResourceType, found, err := WorkerBaseResourceType{
		Name:       baseResourceType.Name,
		WorkerName: sourceWorker,
	}.Find(tx)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, ErrWorkerBaseResourceTypeDisappeared
	}

	ids, workerBaseResourceTypeIds, found, err := workerResourceCache.find(tx)
	if err != nil {
		return nil, false, err
	}

	if found {
		for i, workerBaseResourceTypeId := range workerBaseResourceTypeIds {
			if workerBaseResourceTypeId == usedWorkerBaseResourceType.ID {
				return &UsedWorkerResourceCache{
					ID: ids[i],
				}, true, nil
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
		Suffix(`RETURNING id`).
		RunWith(tx).
		QueryRow().
		Scan(&id)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == pqUniqueViolationErrCode {
			return nil, false, nil
		}
		return nil, false, err
	}

	return &UsedWorkerResourceCache{
		ID: id,
	}, true, nil
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

	// If there are multiple worker resource caches found, choose a random one.
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
