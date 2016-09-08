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

type ResourceCache struct {
	ResourceConfig
	Version atc.Version
}

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
			).
			Values(
				resourceConfig.ID,
				cache.version(),
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
