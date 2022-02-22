package db

import (
	"database/sql"
	"errors"

	sq "github.com/Masterminds/squirrel"
	"github.com/lib/pq"
)

type WorkerResourceCache struct {
	WorkerName    string
	ResourceCache ResourceCache
}

type UsedWorkerResourceCache struct {
	ID                       int
	WorkerBaseResourceTypeID int
}

var ErrWorkerBaseResourceTypeDisappeared = errors.New("worker base resource type disappeared")

// FindOrCreate finds or creates a worker_resource_cache initialized from a
// given sourceWorkerBaseResourceTypeID (which dictates the original worker
// that ran the get step for this resource cache). If there already exists a
// worker_resource_cache for the provided WorkerName and ResourceCache, but
// initialized from a different source worker, it will return `false` as its
// second return value.
//
// This can happen if multiple volumes for the same resource cache are being
// streamed to a worker simultaneously from multiple other "source" workers -
// we only want a single worker_resource_cache in the end for the destination
// worker, so the "first write wins".
func (workerResourceCache WorkerResourceCache) FindOrCreate(tx Tx, sourceWorkerBaseResourceTypeID int) (*UsedWorkerResourceCache, bool, error) {
	uwrc, found, err := workerResourceCache.find(tx)
	if err != nil {
		return nil, false, err
	}
	if found {
		// If the found worker resource cache's worker base resource type id is
		// 0, then means the original source has been invalidated, so we need to
		// reset the worker resource cache with the new base resource type id.
		if uwrc.WorkerBaseResourceTypeID == 0 {
			_, err := psql.Update("worker_resource_caches").
				Set("worker_base_resource_type_id", sourceWorkerBaseResourceTypeID).
				Where(sq.And{
					sq.Eq{"resource_cache_id": workerResourceCache.ResourceCache.ID()},
					sq.Eq{"worker_name": workerResourceCache.WorkerName},
				}).RunWith(tx).Exec()
			if err != nil {
				return nil, false, err
			}
			uwrc.WorkerBaseResourceTypeID = sourceWorkerBaseResourceTypeID
			return uwrc, true, nil
		}

		valid := sourceWorkerBaseResourceTypeID == uwrc.WorkerBaseResourceTypeID
		return uwrc, valid, nil
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
			sourceWorkerBaseResourceTypeID,
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
		ID:                       id,
		WorkerBaseResourceTypeID: sourceWorkerBaseResourceTypeID,
	}, true, nil
}

// Find looks for a worker resource cache by resource cache id and worker name.
// If returned worker_resource_cache's worker_base_resource_type_id is 0, that
// means the source base resource type has been invalidated, thus the cache itself
// can still be used, but it should no longer be cached again on other workers.
func (workerResourceCache WorkerResourceCache) Find(runner sq.Runner) (*UsedWorkerResourceCache, bool, error) {
	uwrc, found, err := workerResourceCache.find(runner)
	return uwrc, found, err
}

func (workerResourceCache WorkerResourceCache) find(runner sq.Runner) (*UsedWorkerResourceCache, bool, error) {
	var id int
	var workerBaseResourceTypeID sql.NullInt64
	err := psql.Select("id", "worker_base_resource_type_id").
		From("worker_resource_caches").
		Where(sq.Eq{
			"resource_cache_id": workerResourceCache.ResourceCache.ID(),
			"worker_name":       workerResourceCache.WorkerName,
		}).
		Suffix("FOR SHARE").
		RunWith(runner).
		QueryRow().
		Scan(&id, &workerBaseResourceTypeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}

	wbrtId := 0
	if workerBaseResourceTypeID.Valid {
		wbrtId = int(workerBaseResourceTypeID.Int64)
	}

	return &UsedWorkerResourceCache{ID: id, WorkerBaseResourceTypeID: wbrtId}, true, nil
}
