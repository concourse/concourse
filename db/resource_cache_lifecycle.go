package db

import (
	"database/sql"
	"strconv"

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
	if err != nil {
		return err
	}

	return nil
}

func (f *resourceCacheLifecycle) CleanUsesForFinishedBuilds(logger lager.Logger) error {
	_, err := psql.Delete("resource_cache_uses rcu USING builds b").
		Where(sq.And{
			sq.Expr("rcu.build_id = b.id"),
			sq.Expr("b.interceptible = false"),
		}).
		RunWith(f.conn).
		Exec()
	if err != nil {
		return err
	}

	return nil
}

func (f *resourceCacheLifecycle) CleanUpInvalidCaches(logger lager.Logger) error {
	stillInUseCacheIds, _, err := sq.
		Select("resource_cache_id").
		Distinct().
		From("resource_cache_uses").
		ToSql()
	if err != nil {
		return err
	}

	logger.Debug("caches-still-in-use", lager.Data{"id": stillInUseCacheIds})

	resourceConfigCacheIds, _, err := sq.
		Select("resource_cache_id").
		Distinct().
		From("resource_configs").
		Where(sq.NotEq{"resource_cache_id": nil}).
		ToSql()
	if err != nil {
		return err
	}

	logger.Debug("caches-for-resource-configs", lager.Data{"id": resourceConfigCacheIds})

	buildImageCacheIds, _, err := sq.
		Select("resource_cache_id").
		Distinct().
		From("build_image_resource_caches").
		ToSql()
	if err != nil {
		return err
	}

	logger.Debug("caches-for-build-images", lager.Data{"id": buildImageCacheIds})

	nextBuildInputsCacheIds, _, err := sq.
		Select("r_cache.id").
		Distinct().
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

	logger.Debug("caches-for-next-build-inputs", lager.Data{"id": nextBuildInputsCacheIds})

	query, args, err := sq.Delete("resource_caches").
		Where("id NOT IN (" + stillInUseCacheIds + ")").
		Where("id NOT IN (" + resourceConfigCacheIds + ")").
		Where("id NOT IN (" + buildImageCacheIds + ")").
		Where("id NOT IN (" + nextBuildInputsCacheIds + ")").
		Suffix("RETURNING id").
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return err
	}

	rows, err := f.conn.Query(query, args...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "foreign_key_violation" {
			// this can happen if a use or resource cache is created referencing the
			// config; as the subqueries above are not atomic
			return nil
		}

		return err
	}

	defer rows.Close()

	var deletedCacheIDs []int
	for rows.Next() {
		var id sql.NullString

		err = rows.Scan(&id)
		if err != nil {
			return nil
		}

		cacheID, err := strconv.Atoi(id.String)
		if err != nil {
			return nil
		}

		deletedCacheIDs = append(deletedCacheIDs, cacheID)
	}

	logger.Debug("deleted-resource-caches", lager.Data{"id": deletedCacheIDs})

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
	if err != nil {
		return err
	}

	return nil
}
