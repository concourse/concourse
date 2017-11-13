package db

import (
	"strings"

	"code.cloudfoundry.org/lager"
	sq "github.com/Masterminds/squirrel"
	"github.com/lib/pq"
)

//go:generate counterfeiter . ResourceCacheLifecycle

type ResourceCacheLifecycle interface {
	CleanUsesForFinishedBuilds(lager.Logger) error
	CleanBuildImageResourceCaches(lager.Logger) error
	CleanUpInvalidCaches(lager.Logger) error
}

type resourceCacheLifecycle struct {
	conn Conn
}

func NewResourceCacheLifecycle(conn Conn) ResourceCacheLifecycle {
	return &resourceCacheLifecycle{
		conn: conn,
	}
}

func (f *resourceCacheLifecycle) CleanBuildImageResourceCaches(logger lager.Logger) error {
	_, err := sq.Delete("build_image_resource_caches birc USING builds b").
		Where("birc.build_id = b.id").
		Where(sq.Expr("((now() - b.end_time) > '24 HOURS'::INTERVAL)")).
		Where(sq.Eq{"job_id": nil}).
		RunWith(f.conn).
		Exec()
	return err
}

func (f *resourceCacheLifecycle) CleanUsesForFinishedBuilds(logger lager.Logger) error {
	_, err := psql.Delete("resource_cache_uses rcu USING builds b").
		Where(sq.And{
			sq.Expr("rcu.build_id = b.id"),
			sq.Expr("b.interceptible = false"),
		}).
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
		Join("versioned_resources vr ON vr.id = nbi.version_id").
		Join("resources r ON r.id = vr.resource_id").
		Join("resource_caches r_cache ON r_cache.version = vr.version").
		Join("resource_configs r_config ON r_cache.resource_config_id = r_config.id").
		Join("jobs j ON nbi.job_id = j.id").
		Join("pipelines p ON j.pipeline_id = p.id").
		Where(sq.Expr("r.resource_config_id = r_config.id")).
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
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == pqFKeyViolationErrCode {
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

func (f *resourceCacheLifecycle) CleanUsesForPausedPipelineResources() error {
	pausedPipelineIds, _, err := sq.
		Select("id").
		Distinct().
		From("pipelines").
		Where(sq.Expr("paused = false")).
		ToSql()
	if err != nil {
		return err
	}

	_, err = psql.Delete("resource_cache_uses rcu USING resources r").
		Where(sq.And{
			sq.Expr("r.pipeline_id NOT IN (" + pausedPipelineIds + ")"),
			sq.Expr("rcu.resource_id = r.id"),
		}).
		RunWith(f.conn).
		Exec()

	return err
}
