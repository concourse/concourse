package dbng

import (
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
)

//go:generate counterfeiter . ResourceCacheFactory

type ResourceCacheFactory interface {
	FindOrCreateResourceCacheForBuild(
		build *Build,
		resourceTypeName string,
		version atc.Version,
		source atc.Source,
		params atc.Params,
		pipeline *Pipeline,
		resourceTypes atc.ResourceTypes,
	) (*UsedResourceCache, error)

	FindOrCreateResourceCacheForResource(
		resource *Resource,
		resourceTypeName string,
		version atc.Version,
		source atc.Source,
		params atc.Params,
		pipeline *Pipeline,
		resourceTypes atc.ResourceTypes,
	) (*UsedResourceCache, error)

	FindOrCreateResourceCacheForResourceType(
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
	conn Conn
}

func NewResourceCacheFactory(conn Conn) ResourceCacheFactory {
	return &resourceCacheFactory{
		conn: conn,
	}
}

func (f *resourceCacheFactory) FindOrCreateResourceCacheForBuild(
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

	dbResourceTypes, err := getDBResourceTypes(tx, pipeline, resourceTypes)
	if err != nil {
		return nil, err
	}

	resourceConfig, err := constructResourceConfig(resourceTypeName, source, dbResourceTypes)
	if err != nil {
		return nil, err
	}

	resourceCache := ResourceCache{
		ResourceConfig: resourceConfig,
		Version:        version,
		Params:         params,
	}

	usedResourceCache, err := resourceCache.FindOrCreateForBuild(tx, build)
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

	dbResourceTypes, err := getDBResourceTypes(tx, pipeline, resourceTypes)
	if err != nil {
		return nil, err
	}

	resourceConfig, err := constructResourceConfig(resourceTypeName, source, dbResourceTypes)
	if err != nil {
		return nil, err
	}

	resourceCache := ResourceCache{
		ResourceConfig: resourceConfig,
		Version:        version,
		Params:         params,
	}

	usedResourceCache, err := resourceCache.FindOrCreateForResource(tx, resource)
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

	dbResourceTypes, err := getDBResourceTypes(tx, pipeline, resourceTypes)
	if err != nil {
		return nil, err
	}

	resourceConfig, err := constructResourceConfig(resourceType.Name, source, dbResourceTypes)
	if err != nil {
		return nil, err
	}

	resourceCache := ResourceCache{
		ResourceConfig: resourceConfig,
		Version:        version,
		Params:         params,
	}

	usedResourceCache, err := resourceCache.FindOrCreateForResourceType(tx, usedResourceType)
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

	_, err = psql.Delete("resource_cache_uses rcu USING builds b").
		Where(sq.And{
			sq.Expr("rcu.build_id = b.id"),
			sq.Or{
				sq.Eq{
					"b.status": "succeeded",
				},
				sq.Eq{
					"b.status": "failed",
				},
				sq.Eq{
					"b.status": "aborted",
				},
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

	latestBuildByJobQ, _, err := sq.
		Select("MAX(b.id) AS build_id", "j.id AS job_id").
		From("builds b").
		Join("jobs j ON j.id = b.job_id").
		GroupBy("j.id").ToSql()
	if err != nil {
		return err
	}

	latestImageResourceVersionsQ, _, err := sq.
		Select("irv.version",
			"rfu.resource_config_id",
			"lbbj.build_id",
			"lbbj.job_id",
			"rc.id AS cache_id",
			"rc.params_hash").
		From("image_resource_versions irv").
		Join("(" + latestBuildByJobQ + ") lbbj ON irv.build_id = lbbj.build_id").
		JoinClause("INNER JOIN resource_config_uses rfu ON rfu.build_id = irv.build_id").
		JoinClause("INNER JOIN resource_caches rc ON rc.resource_config_id = rfu.resource_config_id").
		Where(sq.Expr("rc.params_hash IS NULL")).
		Where(sq.Expr("irv.version = rc.version")).
		ToSql()
	if err != nil {
		return err
	}

	extractedCacheIds, _, err := sq.
		Select("lirvcq.cache_id").
		Distinct().
		From("(" + latestImageResourceVersionsQ + ") as lirvcq").
		ToSql()
	if err != nil {
		return err
	}

	_, err = sq.Delete("resource_caches").
		Where("id NOT IN (" + extractedCacheIds + ")").
		PlaceholderFormat(sq.Dollar).
		RunWith(tx).Exec()

	// fmt.Printf(thing)

	// fmt.Printf(latestImageResourceVersionCachesQ)
	//
	//
	// DELETE FROM resource_caches c
	// WHERE NOT EXISTS (
	// SELECT irv.version, rfu.resource_config_id, foo.jid, rc.id
	// FROM image_resource_versions irv
	// JOIN (SELECT MAX(b.id) as bid, j.id as jid FROM builds b
	// JOIN jobs j ON j.id = b.job_id
	// GROUP BY j.id) foo ON foo.bid = irv.build_id
	// INNER JOIN resource_config_uses rfu ON rfu.build_id = irv.build_id
	// INNER JOIN resource_caches rc ON rc.resource_config_id = rfu.resource_config_id
	// WHERE rc.version = irv.version AND rc.id = c.id AND c.params_hash IS NULL)

	// _, err = psql.Delete("resource_cache_uses rcu USING resources r").
	// 	Where(sq.And{
	// 		sq.Expr("rcu.resource_id = r.id"),
	// 		sq.Eq{
	// 			"r.active": false,
	// 		},
	// 	}).
	// 	RunWith(tx).
	// 	Exec()
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}
