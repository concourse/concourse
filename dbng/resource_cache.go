package dbng

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"code.cloudfoundry.org/lager"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db/lock"
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

func (cache ResourceCache) FindOrCreateForBuild(logger lager.Logger, tx Tx, lockFactory lock.LockFactory, build *Build) (*UsedResourceCache, error) {
	usedResourceConfig, err := cache.ResourceConfig.FindOrCreateForBuild(logger, tx, lockFactory, build)
	if err != nil {
		return nil, err
	}

	return cache.findOrCreate(logger, tx, lockFactory, usedResourceConfig, "build_id", build.ID)
}

func (cache ResourceCache) FindOrCreateForResource(logger lager.Logger, tx Tx, lockFactory lock.LockFactory, resource *Resource) (*UsedResourceCache, error) {
	usedResourceConfig, err := cache.ResourceConfig.FindOrCreateForResource(logger, tx, lockFactory, resource)
	if err != nil {
		return nil, err
	}

	return cache.findOrCreate(logger, tx, lockFactory, usedResourceConfig, "resource_id", resource.ID)
}

func (cache ResourceCache) FindOrCreateForResourceType(logger lager.Logger, tx Tx, lockFactory lock.LockFactory, resourceType *UsedResourceType) (*UsedResourceCache, error) {
	usedResourceConfig, err := cache.ResourceConfig.FindOrCreateForResourceType(logger, tx, lockFactory, resourceType)
	if err != nil {
		return nil, err
	}

	return cache.findOrCreate(logger, tx, lockFactory, usedResourceConfig, "resource_type_id", resourceType.ID)
}

func (cache *UsedResourceCache) Destroy(tx Tx) (bool, error) {
	rows, err := psql.Delete("resource_caches").
		Where(sq.Eq{
			"id": cache.ID,
		}).
		RunWith(tx).
		Exec()
	if err != nil {
		return false, err
	}

	affected, err := rows.RowsAffected()
	if err != nil {
		return false, err
	}

	if affected == 0 {
		panic("TESTME")
		return false, nil
	}

	return true, nil
}

func (cache ResourceCache) findOrCreate(logger lager.Logger, tx Tx, lockFactory lock.LockFactory, resourceConfig *UsedResourceConfig, forColumnName string, forColumnID int) (*UsedResourceCache, error) {
	id, found, err := cache.findWithResourceConfig(tx, resourceConfig)
	if err != nil {
		return nil, err
	}

	if !found {
		lockName, err := cache.lockName()
		if err != nil {
			return nil, err
		}

		lock := lockFactory.NewLock(
			logger.Session("find-or-create-resource-cache"),
			lock.NewTaskLockID(lockName),
		)

		acquired, err := lock.Acquire()
		if err != nil {
			return nil, err
		}

		if !acquired {
			return cache.findOrCreate(logger, tx, lockFactory, resourceConfig, forColumnName, forColumnID)
		}

		defer lock.Release()

		id, found, err = cache.findWithResourceConfig(tx, resourceConfig)
		if err != nil {
			return nil, err
		}

		if !found {
			err = psql.Insert("resource_caches").
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
				if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "foreign_key_violation" {
					return nil, ErrResourceCacheConfigDisappeared
				}

				return nil, err
			}
		}

		lock.Release()
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

func (cache ResourceCache) lockName() (string, error) {
	cacheJSON, err := json.Marshal(cache)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(cacheJSON)), nil
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
