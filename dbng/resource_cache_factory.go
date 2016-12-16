package dbng

import (
	"code.cloudfoundry.org/lager"
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db/lock"
)

//go:generate counterfeiter . ResourceCacheFactory

type ResourceCacheFactory interface {
	FindOrCreateResourceCacheForBuild(
		logger lager.Logger,
		build *Build,
		resourceTypeName string,
		version atc.Version,
		source atc.Source,
		params atc.Params,
		pipeline *Pipeline,
		resourceTypes atc.ResourceTypes,
	) (*UsedResourceCache, error)

	FindOrCreateResourceCacheForResource(
		logger lager.Logger,
		resource *Resource,
		resourceTypeName string,
		version atc.Version,
		source atc.Source,
		params atc.Params,
		pipeline *Pipeline,
		resourceTypes atc.ResourceTypes,
	) (*UsedResourceCache, error)

	FindOrCreateResourceCacheForResourceType(
		logger lager.Logger,
		resourceTypeName string,
		version atc.Version,
		source atc.Source,
		params atc.Params,
		pipeline *Pipeline,
		resourceTypes atc.ResourceTypes,
	) (*UsedResourceCache, error)

	CleanUsesForFinishedBuilds() error
	CleanUsesForInactiveResourceTypes() error
	CleanUsesForInactiveResources() error

	CleanUpInvalidCaches() error
}

type resourceCacheFactory struct {
	conn        Conn
	lockFactory lock.LockFactory
}

func NewResourceCacheFactory(conn Conn, lockFactory lock.LockFactory) ResourceCacheFactory {
	return &resourceCacheFactory{
		conn:        conn,
		lockFactory: lockFactory,
	}
}

func (f *resourceCacheFactory) FindOrCreateResourceCacheForBuild(
	logger lager.Logger,
	build *Build,
	resourceTypeName string,
	version atc.Version,
	source atc.Source,
	params atc.Params,
	pipeline *Pipeline,
	resourceTypes atc.ResourceTypes,
) (*UsedResourceCache, error) {
	tx, err := f.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	resourceConfig, err := constructResourceConfig(tx, resourceTypeName, source, resourceTypes, pipeline)
	if err != nil {
		return nil, err
	}

	resourceCache := ResourceCache{
		ResourceConfig: resourceConfig,
		Version:        version,
		Params:         params,
	}

	usedResourceCache, err := resourceCache.FindOrCreateForBuild(logger, tx, f.lockFactory, build)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return usedResourceCache, nil
}

func (f *resourceCacheFactory) FindOrCreateResourceCacheForResource(
	logger lager.Logger,
	resource *Resource,
	resourceTypeName string,
	version atc.Version,
	source atc.Source,
	params atc.Params,
	pipeline *Pipeline,
	resourceTypes atc.ResourceTypes,
) (*UsedResourceCache, error) {
	tx, err := f.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	resourceConfig, err := constructResourceConfig(tx, resourceTypeName, source, resourceTypes, pipeline)
	if err != nil {
		return nil, err
	}

	resourceCache := ResourceCache{
		ResourceConfig: resourceConfig,
		Version:        version,
		Params:         params,
	}

	usedResourceCache, err := resourceCache.FindOrCreateForResource(logger, tx, f.lockFactory, resource)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return usedResourceCache, nil
}

func (f *resourceCacheFactory) FindOrCreateResourceCacheForResourceType(
	logger lager.Logger,
	resourceTypeName string,
	version atc.Version,
	source atc.Source,
	params atc.Params,
	pipeline *Pipeline,
	resourceTypes atc.ResourceTypes,
) (*UsedResourceCache, error) {
	resourceType, found := resourceTypes.Lookup(resourceTypeName)
	if !found {
		return nil, ErrResourceTypeNotFound{resourceTypeName}
	}

	tx, err := f.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	rt := ResourceType{
		ResourceType: resourceType,
		Pipeline:     pipeline,
	}

	usedResourceType, found, err := rt.Find(tx)
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, ErrResourceTypeNotFound{resourceTypeName}
	}

	resourceConfig, err := constructResourceConfig(tx, resourceType.Name, source, resourceTypes, pipeline)
	if err != nil {
		return nil, err
	}

	resourceCache := ResourceCache{
		ResourceConfig: resourceConfig,
		Version:        version,
		Params:         params,
	}

	usedResourceCache, err := resourceCache.FindOrCreateForResourceType(logger, tx, f.lockFactory, usedResourceType)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return usedResourceCache, nil
}

func (f *resourceCacheFactory) CleanUsesForFinishedBuilds() error {
	tx, err := f.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	latestImageResourceBuildByJobQ, _, err := sq.
		Select("MAX(b.id) AS max_build_id").
		From("image_resource_versions irv").
		Join("builds b ON b.id = irv.build_id").
		Where(sq.NotEq{"b.job_id": nil}).
		GroupBy("b.job_id").ToSql()
	if err != nil {
		return err
	}

	imageResourceCacheIds, _, err := sq.
		Select("rc.id").
		From("image_resource_versions irv").
		Join("resource_caches rc ON rc.version = irv.version").
		Join("resource_cache_uses rcu ON rcu.resource_cache_id = rc.id").
		Where(sq.Expr("irv.build_id = rcu.build_id")).
		Where(sq.Expr("rc.params_hash = 'null'")).
		Where("irv.build_id IN (" + latestImageResourceBuildByJobQ + ")").
		ToSql()
	if err != nil {
		return err
	}

	latestBuildByJobQ, _, err := sq.
		Select("MAX(b.id) AS build_id", "j.id AS job_id").
		From("builds b").
		Join("jobs j ON j.id = b.job_id").
		GroupBy("j.id").ToSql()
	if err != nil {
		return err
	}

	extractedBuildIds, _, err := sq.
		Select("lbbjq.build_id").
		Distinct().
		From("(" + latestBuildByJobQ + ") as lbbjq").
		ToSql()
	if err != nil {
		return err
	}

	_, err = psql.Delete("resource_cache_uses rcu USING builds b").
		Where(sq.And{
			sq.Expr("rcu.build_id = b.id"),
			sq.Or{
				sq.Eq{
					"b.status": "succeeded",
				},
				sq.And{
					sq.Expr("b.id NOT IN (" + extractedBuildIds + ")"),
					sq.Eq{
						"b.status": "failed",
					},
				},
				sq.Eq{
					"b.status": "aborted",
				},
			},
		}).
		Where("rcu.resource_cache_id NOT IN (" + imageResourceCacheIds + ")").
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (f *resourceCacheFactory) CleanUsesForInactiveResourceTypes() error {
	tx, err := f.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = psql.Delete("resource_cache_uses rcu USING resource_types t").
		Where(sq.And{
			sq.Expr("rcu.resource_type_id = t.id"),
			sq.Eq{
				"t.active": false,
			},
		}).
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (f *resourceCacheFactory) CleanUsesForInactiveResources() error {
	tx, err := f.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = psql.Delete("resource_cache_uses rcu USING resources r").
		Where(sq.And{
			sq.Expr("rcu.resource_id = r.id"),
			sq.Eq{
				"r.active": false,
			},
		}).
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (f *resourceCacheFactory) CleanUpInvalidCaches() error {
	tx, err := f.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stillInUseCacheIds, _, err := sq.
		Select("rc.id").
		Distinct().
		From("resource_caches rc").
		Join("resource_cache_uses rcu ON rc.id = rcu.resource_cache_id").
		ToSql()
	if err != nil {
		return err
	}

	nextBuildInputsCacheIds, _, err := sq.
		Select("rc.id").
		Distinct().
		From("next_build_inputs nbi").
		Join("versioned_resources vr ON vr.id = nbi.version_id").
		Join("resources r ON r.id = vr.resource_id").
		Join("resource_caches rc ON rc.version = vr.version").
		Join("resource_configs rf ON rc.resource_config_id = rf.id").
		Where(sq.Expr("r.config::json->>'source' = rf.source_hash")).
		ToSql()
	if err != nil {
		return err
	}

	_, err = sq.Delete("resource_caches").
		Where("id NOT IN (" + nextBuildInputsCacheIds + ")").
		Where("id NOT IN (" + stillInUseCacheIds + ")").
		PlaceholderFormat(sq.Dollar).
		RunWith(tx).Exec()
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}
