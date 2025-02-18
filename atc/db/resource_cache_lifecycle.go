package db

import (
	"strings"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	"code.cloudfoundry.org/lager/v3"
	sq "github.com/Masterminds/squirrel"
)

//counterfeiter:generate . ResourceCacheLifecycle
type ResourceCacheLifecycle interface {
	CleanUsesForFinishedBuilds(lager.Logger) error
	CleanBuildImageResourceCaches(lager.Logger) error
	CleanUpInvalidCaches(lager.Logger) error
	CleanInvalidWorkerResourceCaches(lager.Logger, int) error
	CleanDirtyInMemoryBuildUses(lager.Logger) error
}

type resourceCacheLifecycle struct {
	conn DbConn
}

func NewResourceCacheLifecycle(conn DbConn) ResourceCacheLifecycle {
	return &resourceCacheLifecycle{
		conn: conn,
	}
}

func (f *resourceCacheLifecycle) CleanBuildImageResourceCaches(lager.Logger) error {
	_, err := sq.Delete("build_image_resource_caches birc USING builds b").
		Where("birc.build_id = b.id").
		Where(sq.Expr("((now() - b.end_time) > '24 HOURS'::INTERVAL)")).
		Where(sq.Eq{"birc.job_id": nil}).
		RunWith(f.conn).
		Exec()
	return err
}

func (f *resourceCacheLifecycle) CleanUsesForFinishedBuilds(lager.Logger) error {
	_, err := psql.Delete("resource_cache_uses rcu USING builds b").
		Where(sq.And{
			sq.Expr("rcu.build_id = b.id"),
			sq.Expr("b.interceptible = false"),
		}).
		RunWith(f.conn).
		Exec()
	return err
}

func (f *resourceCacheLifecycle) CleanDirtyInMemoryBuildUses(lager.Logger) error {
	_, err := sq.Delete("resource_cache_uses").
		Where(sq.Expr("((now() - in_memory_build_create_time) > '24 HOURS'::INTERVAL)")).
		RunWith(f.conn).
		Exec()
	return err
}

func (f *resourceCacheLifecycle) CleanInvalidWorkerResourceCaches(logger lager.Logger, batchSize int) error {
	_, err := sq.Delete("worker_resource_caches").
		Where(sq.Expr("id in (SELECT id FROM invalid_caches)")).
		Prefix(
			`WITH invalid_caches AS (
				    SELECT id FROM worker_resource_caches 
				    WHERE worker_base_resource_type_id IS NULL AND 
				        invalid_since < (
				            SELECT COALESCE(MIN(start_time), now()) FROM builds WHERE status = 'started'
				        )
				    LIMIT $1
			  )`, batchSize).
		RunWith(f.conn).
		Exec()
	return err
}

func (f *resourceCacheLifecycle) CleanUpInvalidCaches(logger lager.Logger) error {
	stillInUseCacheIds, _, err := sq.
		Select("resource_cache_id").
		From("resource_cache_uses").
		ToSql()
	if err != nil {
		return err
	}

	resourceConfigCacheIds, _, err := sq.
		Select("resource_cache_id").
		From("resource_configs").
		Where(sq.NotEq{"resource_cache_id": nil}).
		ToSql()
	if err != nil {
		return err
	}

	buildImageCacheIds, _, err := sq.
		Select("resource_cache_id").
		From("build_image_resource_caches").
		ToSql()
	if err != nil {
		return err
	}

	nextBuildInputsCacheIds, _, err := sq.
		Select("r_cache.id").
		From("next_build_inputs nbi").
		Join("resources r ON r.id = nbi.resource_id").
		Join("resource_config_versions rcv ON rcv.version_sha256 = nbi.version_sha256 AND rcv.resource_config_scope_id = r.resource_config_scope_id").
		Join("resource_caches r_cache ON r_cache.resource_config_id = r.resource_config_id AND r_cache.version_sha256 = rcv.version_sha256").
		Join("jobs j ON nbi.job_id = j.id").
		Join("pipelines p ON j.pipeline_id = p.id").
		Where(sq.Expr("p.paused = false")).
		ToSql()
	if err != nil {
		return err
	}

	query, args, err := sq.Delete("resource_caches").
		Where("id NOT IN (" + strings.Join([]string{
			stillInUseCacheIds,
			resourceConfigCacheIds,
			buildImageCacheIds,
			nextBuildInputsCacheIds,
		}, " UNION ") + ")").
		Suffix("RETURNING id").
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return err
	}

	rows, err := f.conn.Query(query, args...)
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == pgerrcode.ForeignKeyViolation {
			// this can happen if a use or resource cache is created referencing the
			// config; as the subqueries above are not atomic
			return nil
		}

		return err
	}

	defer Close(rows)

	var deletedCacheIDs []int
	for rows.Next() {
		var cacheID int
		err = rows.Scan(&cacheID)
		if err != nil {
			return nil
		}

		deletedCacheIDs = append(deletedCacheIDs, cacheID)
	}

	if len(deletedCacheIDs) > 0 {
		logger.Debug("deleted-resource-caches", lager.Data{"id": deletedCacheIDs})
	}

	return nil
}
