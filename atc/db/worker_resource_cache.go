package db

import (
	"database/sql"
	"errors"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"time"

	sq "github.com/Masterminds/squirrel"
)

// WorkerResourceCache stores resource caches on each worker. WorkerBaseResourceTypeID
// field records the original worker's base resource id when the cache is created. For
// example, when a resource cache is created on worker-1 and WorkerBaseResourceTypeID
// is 100. Then the resource cache is streamed to worker-2, a new worker resource cache
// will be created for worker-2, and WorkerBaseResourceTypeID will still be 100.
//
// If worker-1 is pruned, the worker resource cache on worker-2 will be invalidated by
// setting WorkerBaseResourceTypeID to 0 and invalid_since to current time. Thus, a worker
// resource cache is called "valid" when its WorkerBaseResourceTypeID is not 0, and called
// "invalidated" when its WorkerBaseResourceTypeID is 0.
//
// Builds started before invalid_since of an invalidated worker resource cache will still
// be able to use the cache. But if the cache is streamed to other workers, streamed
// volumes should no longer be marked as cache again.
//
// When there is no running build started before invalid_since of an invalidated cache,
// the cache will be GC-ed.
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
	uwrc, found, err := workerResourceCache.find(tx, nil, true)
	if err != nil {
		return nil, false, err
	}
	if found {
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
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == pgerrcode.UniqueViolation {
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
// (worker_base_resource_type_id is 0) might be returned, but the invalidated
// cache's invalid_since must be later than volumeShouldBeValidBefore.
func (workerResourceCache WorkerResourceCache) Find(runner sq.Runner, volumeShouldBeValidBefore time.Time) (*UsedWorkerResourceCache, bool, error) {
	return workerResourceCache.find(runner, &volumeShouldBeValidBefore, false)
}

// FindByID looks for a worker resource cache by resource cache id. To init a
// streamed volume as cache, it should check to see if the original cache is
// still valid.
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

// find should return a valid worker resource cache if its WorkerBaseResourceTypeID is not null.
// When volumeShouldBeValidBefore find should only return a valid worker resource cache; otherwise
// find may return an invalidated worker resource cache whose WorkerBaseResourceTypeID is null and
// invalid_since is later than volumeShouldBeValidBefore. If there are multiple invalidated caches
// with invalid_since later than volumeShouldBeValidBefore, the invalidated cache with newest
// invalid_since should be returned.
func (workerResourceCache WorkerResourceCache) find(runner sq.Runner, volumeShouldBeValidBefore *time.Time, forShare bool) (*UsedWorkerResourceCache, bool, error) {
	var id int
	var workerBaseResourceTypeID sql.NullInt64
	var invalidSince sql.NullTime

	sb := psql.Select("id", "worker_base_resource_type_id", "invalid_since").
		From("worker_resource_caches").
		Where(sq.Expr("id = (select id from best_cache)")).
		Prefix(
			`WITH candidates AS (
                     SELECT id, worker_base_resource_type_id, invalid_since
                       FROM worker_resource_caches
                       WHERE resource_cache_id = $1 and worker_name = $2
                 ),
				 valid_caches AS (
                     SELECT id, invalid_since, 1 as priority
                       FROM candidates
                       WHERE worker_base_resource_type_id is not null
				     UNION ALL
				     SELECT id, invalid_since, 2 as priority
                       FROM candidates
                       WHERE worker_base_resource_type_id is null
                       ORDER BY invalid_since DESC
                       LIMIT 1
                 ),
                 best_cache as (
                     select id from valid_caches order by priority limit 1
                 )`,
			workerResourceCache.ResourceCache.ID(), workerResourceCache.WorkerName,
		)
	if forShare {
		sb = sb.Suffix("FOR SHARE")
	}
	err := sb.RunWith(runner).QueryRow().Scan(&id, &workerBaseResourceTypeID, &invalidSince)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}

	if workerBaseResourceTypeID.Valid {
		return &UsedWorkerResourceCache{
			ID:                       id,
			WorkerBaseResourceTypeID: int(workerBaseResourceTypeID.Int64),
		}, true, nil
	}

	if volumeShouldBeValidBefore == nil || invalidSince.Valid && invalidSince.Time.Before(*volumeShouldBeValidBefore) {
		return nil, false, nil
	}

	return &UsedWorkerResourceCache{
		ID:                       id,
		WorkerBaseResourceTypeID: 0,
	}, true, nil
}
