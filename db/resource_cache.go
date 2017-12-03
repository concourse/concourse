package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"code.cloudfoundry.org/lager"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/lib/pq"
)

var ErrResourceCacheAlreadyExists = errors.New("resource-cache-already-exists")
var ErrResourceCacheDisappeared = errors.New("resource-cache-disappeared")

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
	Version        atc.Version
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
		return false, ErrResourceCacheDisappeared
	}

	return true, nil
}

func (cache ResourceCache) Find(tx Tx) (*UsedResourceCache, bool, error) {
	usedResourceConfig, found, err := cache.ResourceConfig.Find(tx)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	return cache.findWithResourceConfig(tx, usedResourceConfig)
}

func (cache ResourceCache) findOrCreate(
	logger lager.Logger,
	tx Tx,
) (*UsedResourceCache, error) {
	usedResourceConfig, err := cache.ResourceConfig.findOrCreate(logger, tx)
	if err != nil {
		return nil, err
	}

	rc, found, err := cache.findWithResourceConfig(tx, usedResourceConfig)
	if err != nil {
		return nil, err
	}

	if !found {
		var id int
		err = psql.Insert("resource_caches").
			Columns(
				"resource_config_id",
				"version",
				"params_hash",
			).
			Values(
				usedResourceConfig.ID,
				cache.version(),
				paramsHash(cache.Params),
			).
			Suffix("RETURNING id").
			RunWith(tx).
			QueryRow().
			Scan(&id)
		if err != nil {
			if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == pqFKeyViolationErrCode {
				return nil, ErrSafeRetryFindOrCreate
			}

			if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == pqUniqueViolationErrCode {
				return nil, ErrSafeRetryFindOrCreate
			}

			return nil, err
		}

		rc = &UsedResourceCache{
			ID:             id,
			ResourceConfig: usedResourceConfig,
			Version:        cache.Version,
		}
	}

	return rc, nil
}

func (cache ResourceCache) use(
	logger lager.Logger,
	tx Tx,
	rc *UsedResourceCache,
	user ResourceCacheUser,
) error {
	cols := user.SQLMap()
	cols["resource_cache_id"] = rc.ID

	var resourceCacheUseExists int
	err := psql.Select("1").
		From("resource_cache_uses").
		Where(sq.Eq(cols)).
		RunWith(tx).
		QueryRow().
		Scan(&resourceCacheUseExists)
	if err != nil {
		if err != sql.ErrNoRows {
			return err
		}
	}

	if err == nil {
		// use already exists
		return nil
	}

	_, err = psql.Insert("resource_cache_uses").
		SetMap(cols).
		RunWith(tx).
		Exec()
	return err
}

func (cache ResourceCache) findWithResourceConfig(tx Tx, resourceConfig *UsedResourceConfig) (*UsedResourceCache, bool, error) {
	var id int
	err := psql.Select("id").From("resource_caches").Where(sq.Eq{
		"resource_config_id": resourceConfig.ID,
		"version":            cache.version(),
		"params_hash":        paramsHash(cache.Params),
	}).Suffix("FOR SHARE").RunWith(tx).QueryRow().Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}

		return nil, false, err
	}

	return &UsedResourceCache{
		ID:             id,
		ResourceConfig: resourceConfig,
		Version:        cache.Version,
	}, true, nil
}

func (cache ResourceCache) version() string {
	j, _ := json.Marshal(cache.Version)
	return string(j)
}

func (cache *UsedResourceCache) BaseResourceType() *UsedBaseResourceType {
	if cache.ResourceConfig.CreatedByBaseResourceType != nil {
		return cache.ResourceConfig.CreatedByBaseResourceType
	}

	return cache.ResourceConfig.CreatedByResourceCache.BaseResourceType()
}

func paramsHash(p atc.Params) string {
	if p != nil {
		return mapHash(p)
	}

	return mapHash(atc.Params{})
}

func mapHash(m map[string]interface{}) string {
	j, _ := json.Marshal(m)
	return fmt.Sprintf("%x", sha256.Sum256(j))
}
