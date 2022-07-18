package db

import (
	"database/sql"
	"errors"
	"time"

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
	if found && uwrc.WorkerBaseResourceTypeID != 0 {
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
// If there is a valid cache, it will return it; otherwise an invalidated cache
// (worker_base_resource_type_id is 0) might be returned. Different is that if a
// valid worker resource cache is streamed to another worker, then streamed volume
// can be marked as a cache on dest worker. An invalidated worker resource cache
// can still be used by builds, but streamed volumed should not be marked as cache.
func (workerResourceCache WorkerResourceCache) Find(runner sq.Runner) (*UsedWorkerResourceCache, bool, error) {
	uwrc, found, err := workerResourceCache.find(runner)
	return uwrc, found, err
}

// FindExact looks for a worker resource cache by resource cache id, worker name
// and worker_base_resource_type_id. To init a streamed volume as cache, it should
// check to see if the original cache is still valid.
func (workerResourceCache WorkerResourceCache) FindByID(runner sq.Runner, id int) (*UsedWorkerResourceCache, bool, error) {
	var sqWorkerBaseResourceTypeID sql.NullInt64
	err := psql.Select("worker_base_resource_type_id").
		From("worker_resource_caches").
		Where(sq.Eq{"id": id}).
		RunWith(runner).
		QueryRow().Scan(&sqWorkerBaseResourceTypeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}

	var workerBasedResourceTypeID int
	if sqWorkerBaseResourceTypeID.Valid {
		workerBasedResourceTypeID = int(sqWorkerBaseResourceTypeID.Int64)
	}

	return &UsedWorkerResourceCache{
		ID:                       id,
		WorkerBaseResourceTypeID: workerBasedResourceTypeID,
	}, true, nil
}

func (workerResourceCache WorkerResourceCache) find(runner sq.Runner) (*UsedWorkerResourceCache, bool, error) {
	var id int
	var workerBaseResourceTypeID sql.NullInt64
	var invalidSince pq.NullTime

	var idOfNewestInvalid int
	var invalidSinceOfNewestInvalid time.Time

	rows, err := psql.Select("id", "worker_base_resource_type_id", "invalid_since").
		From("worker_resource_caches").
		Where(sq.Eq{
			"resource_cache_id": workerResourceCache.ResourceCache.ID(),
			"worker_name":       workerResourceCache.WorkerName,
		}).
		Suffix("FOR SHARE").
		RunWith(runner).
		Query()
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	for rows.Next() {
		err := rows.Scan(&id, &workerBaseResourceTypeID, &invalidSince)
		if err != nil {
			return nil, false, err
		}

		wbrtId := 0
		if workerBaseResourceTypeID.Valid {
			wbrtId = int(workerBaseResourceTypeID.Int64)
		}

		if wbrtId != 0 {
			// There should be only 1 valid worker resource cache of a resource per worker.
			return &UsedWorkerResourceCache{ID: id, WorkerBaseResourceTypeID: wbrtId}, true, nil
		} else {
			if invalidSince.Time.After(invalidSinceOfNewestInvalid) {
				idOfNewestInvalid = id
				invalidSinceOfNewestInvalid = invalidSince.Time
			}
		}
	}

	if idOfNewestInvalid != 0 {
		return &UsedWorkerResourceCache{
			ID:                       idOfNewestInvalid,
			WorkerBaseResourceTypeID: 0,
		}, true, nil
	}

	return nil, false, nil
}
