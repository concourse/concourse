package dbng

import (
	"database/sql"
	"encoding/json"
	"errors"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/lib/pq"
)

var ErrResourceCacheAlreadyExists = errors.New("resource cache already exists")
var ErrResourceCacheDisappeared = errors.New("resource cache disappeared")
var ErrResourceCacheConfigDisappeared = errors.New("resource cache config disappeared")

// ResourceCache represents an instance of a ResourceConfig's version.
//
// A ResourceCache is created by a `get`, an `image_resource`, or a resource
// type in a pipeline.
//
// ResourceCaches are garbage-collected by gc.ResourceCacheCollector.
type ResourceCache struct {
	ResourceConfig ResourceConfig // The resource configuration.
	Version        atc.Version    // The version of the resource.
	Params         atc.Params     // The params used when fetching the version.
}

// UsedResourceCache is created whenever a ResourceCache is Created and/or
// Used.
//
// So long as the UsedResourceCache exists, the underlying ResourceCache can
// not be removed.
//
// UsedResourceCaches become unused by the gc.ResourceCacheCollector, which may
// then lead to the ResourceCache being garbage-collected.
//
// See FindOrCreateForBuild, FindOrCreateForResource, and
// FindOrCreateForResourceType for more information on when it becomes unused.
type UsedResourceCache struct {
	ID             int
	ResourceConfig *UsedResourceConfig
}

func (cache ResourceCache) FindOrCreateForBuild(tx Tx, build *Build) (*UsedResourceCache, error) {
	usedResourceConfig, err := cache.ResourceConfig.FindOrCreateForBuild(tx, build)
	if err != nil {
		return nil, err
	}

	return cache.findOrCreate(tx, usedResourceConfig, "build_id", build.ID)
}

func (cache ResourceCache) FindOrCreateForResource(tx Tx, resource *Resource) (*UsedResourceCache, error) {
	usedResourceConfig, err := cache.ResourceConfig.FindOrCreateForResource(tx, resource)
	if err != nil {
		return nil, err
	}

	return cache.findOrCreate(tx, usedResourceConfig, "resource_id", resource.ID)
}

func (cache ResourceCache) FindOrCreateForResourceType(tx Tx, resourceType *ResourceType) (*UsedResourceCache, error) {
	usedResourceConfig, err := cache.ResourceConfig.FindOrCreateForResourceType(tx, resourceType)
	if err != nil {
		return nil, err
	}

	return cache.findOrCreate(tx, usedResourceConfig, "resource_type_id", resourceType.ID)
}

func (cache ResourceCache) findOrCreate(tx Tx, resourceConfig *UsedResourceConfig, forColumnName string, forColumnID int) (*UsedResourceCache, error) {
	id, found, err := cache.findWithResourceConfig(tx, resourceConfig)
	if err != nil {
		return nil, err
	}

	if !found {
		err := psql.Insert("resource_caches").
			Columns(
				"resource_config_id",
				"version",
				"params_hash",
			).
			Values(
				resourceConfig.ID,
				cache.version(),
				cache.paramsHash(),
			).
			Suffix("RETURNING id").
			RunWith(tx).
			QueryRow().
			Scan(&id)
		if err != nil {
			if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "unique_violation" {
				return nil, ErrResourceCacheAlreadyExists
			}

			if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "foreign_key_violation" {
				return nil, ErrResourceCacheConfigDisappeared
			}

			return nil, err
		}
	}

	_, err = psql.Insert("resource_cache_uses").
		Columns(
			"resource_cache_id",
			forColumnName,
		).
		Values(
			id,
			forColumnID,
		).
		RunWith(tx).
		Exec()
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "foreign_key_violation" {
			return nil, ErrResourceCacheDisappeared
		}

		return nil, err
	}

	// TODO: rename to ResourceVersion?
	return &UsedResourceCache{
		ID:             id,
		ResourceConfig: resourceConfig,
	}, nil
}

func (cache ResourceCache) findWithResourceConfig(tx Tx, resourceConfig *UsedResourceConfig) (int, bool, error) {
	var id int
	err := psql.Select("id").From("resource_caches").Where(sq.Eq{
		"resource_config_id": resourceConfig.ID,
		"version":            cache.version(),
	}).RunWith(tx).QueryRow().Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, false, nil
		}

		return 0, false, err
	}

	return id, true, nil
}

func (cache ResourceCache) version() string {
	j, _ := json.Marshal(cache.Version)
	return string(j)
}

func (cache ResourceCache) paramsHash() string {
	j, _ := json.Marshal(cache.Params)
	return string(j) // TODO: actually hash
}
