package db

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/lock"
)

var ErrResourceCacheAlreadyExists = errors.New("resource-cache-already-exists")
var ErrResourceCacheDisappeared = errors.New("resource-cache-disappeared")

// ResourceCache represents an instance of a ResourceConfig's version.
//
// A ResourceCache is created by a `get`, an `image_resource`, or a resource
// type in a pipeline.
//
// ResourceCaches are garbage-collected by gc.ResourceCacheCollector.

//counterfeiter:generate . ResourceCache
type ResourceCache interface {
	ID() int
	Version() atc.Version

	ResourceConfig() ResourceConfig

	Destroy(Tx) (bool, error)
	BaseResourceType() *UsedBaseResourceType
}

type resourceCache struct {
	id             int
	resourceConfig ResourceConfig
	version        atc.Version

	lockFactory lock.LockFactory
	conn        DbConn
}

func (cache *resourceCache) ID() int                        { return cache.id }
func (cache *resourceCache) ResourceConfig() ResourceConfig { return cache.resourceConfig }
func (cache *resourceCache) Version() atc.Version           { return cache.version }

func (cache *resourceCache) Destroy(tx Tx) (bool, error) {
	rows, err := psql.Delete("resource_caches").
		Where(sq.Eq{
			"id": cache.id,
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

func (cache *resourceCache) BaseResourceType() *UsedBaseResourceType {
	if cache.resourceConfig.CreatedByBaseResourceType() != nil {
		return cache.resourceConfig.CreatedByBaseResourceType()
	}

	return cache.resourceConfig.CreatedByResourceCache().BaseResourceType()
}

func mapHash(m map[string]interface{}) string {
	j, _ := json.Marshal(m)
	return fmt.Sprintf("%x", sha256.Sum256(j))
}

func paramsHash(p atc.Params) string {
	if p != nil {
		return mapHash(p)
	}

	return mapHash(atc.Params{})
}
