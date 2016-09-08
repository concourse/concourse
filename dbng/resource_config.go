package dbng

import (
	"database/sql"
	"encoding/json"
	"errors"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/lib/pq"
)

var ErrResourceConfigAlreadyExists = errors.New("resource config already exists")
var ErrResourceConfigDisappeared = errors.New("resource config disappeared")
var ErrResourceConfigParentDisappeared = errors.New("resource config parent disappeared")

// ResourceConfig represents a resource type and config source.
//
// Resources in a pipeline, resource types in a pipeline, and `image_resource`
// fields in a task all result in a reference to a ResourceConfig.
//
// ResourceConfigs are garbage-collected by gc.ResourceConfigCollector.
type ResourceConfig struct {
	// A resource type provided by a resource.
	CreatedByResourceCache *ResourceCache

	// A resource type provided by a worker.
	CreatedByBaseResourceType *BaseResourceType

	// The resource's source configuration.
	Source atc.Source
}

// UsedResourceConfig is created whenever a ResourceConfig is Created and/or
// Used.
//
// So long as the UsedResourceConfig exists, the underlying ResourceConfig can
// not be removed.
//
// UsedResourceConfigs become unused by the gc.ResourceConfigCollector, which
// may then lead to the ResourceConfig being garbage-collected.
//
// See FindOrCreateForBuild, FindOrCreateForResource, and
// FindOrCreateForResourceType for more information on when it becomes unused.
type UsedResourceConfig struct {
	ID                        int
	CreatedByResourceCache    *UsedResourceCache
	CreatedByBaseResourceType *UsedBaseResourceType
}

// FindOrCreateForBuild creates the ResourceConfig, recursively creating its
// parent ResourceConfig or BaseResourceType, and registers a "Use" for the
// given build.
//
// An `image_resource` or a `get` within a build will result in a
// UsedResourceConfig.
//
// ErrResourceConfigDisappeared may be returned if the resource config was
// found initially but was removed before we could use it.
//
// ErrResourceConfigAlreadyExists may be returned if a concurrent call resulted
// in a conflict.
//
// ErrResourceConfigParentDisappeared may be returned if the resource config's
// parent ResourceConfig or BaseResourceType was found initially but was
// removed before we could create the ResourceConfig.
//
// Each of these errors should result in the caller retrying from the start of
// the transaction.
func (resourceConfig ResourceConfig) FindOrCreateForBuild(tx Tx, build *Build) (*UsedResourceConfig, error) {
	var resourceCacheID int
	if resourceConfig.CreatedByResourceCache != nil {
		createdByResourceCache, err := resourceConfig.CreatedByResourceCache.FindOrCreateForBuild(tx, build)
		if err != nil {
			return nil, err
		}

		resourceCacheID = createdByResourceCache.ID
	}

	return resourceConfig.findOrCreate(tx, "build_id", build.ID, resourceCacheID)
}

// FindOrCreateForResource creates the ResourceConfig, recursively creating its
// parent ResourceConfig or BaseResourceType, and registers a "Use" for the
// given resource.
//
// A periodic check for a pipeline's resource will result in a
// UsedResourceConfig.
//
// ErrResourceConfigDisappeared may be returned if the resource config was
// found initially but was removed before we could use it.
//
// ErrResourceConfigAlreadyExists may be returned if a concurrent call resulted
// in a conflict.
//
// ErrResourceConfigParentDisappeared may be returned if the resource config's
// parent ResourceConfig or BaseResourceType was found initially but was
// removed before we could create the ResourceConfig.
//
// Each of these errors should result in the caller retrying from the start of
// the transaction.
func (resourceConfig ResourceConfig) FindOrCreateForResource(tx Tx, resource *Resource) (*UsedResourceConfig, error) {
	var resourceCacheID int
	if resourceConfig.CreatedByResourceCache != nil {
		createdByResourceCache, err := resourceConfig.CreatedByResourceCache.FindOrCreateForResource(tx, resource)
		if err != nil {
			return nil, err
		}

		resourceCacheID = createdByResourceCache.ID
	}
	return resourceConfig.findOrCreate(tx, "resource_id", resource.ID, resourceCacheID)
}

// FindOrCreateForResourceType creates the ResourceConfig, recursively creating
// its parent ResourceConfig or BaseResourceType, and registers a "Use" for the
// given resource type.
//
// A periodic check for a pipeline's resource type will result in a
// UsedResourceConfig.
//
// ErrResourceConfigDisappeared may be returned if the resource config was
// found initially but was removed before we could use it.
//
// ErrResourceConfigAlreadyExists may be returned if a concurrent call resulted
// in a conflict.
//
// ErrResourceConfigParentDisappeared may be returned if the resource config's
// parent ResourceConfig or BaseResourceType was found initially but was
// removed before we could create the ResourceConfig.
//
// Each of these errors should result in the caller retrying from the start of
// the transaction.
func (resourceConfig ResourceConfig) FindOrCreateForResourceType(tx Tx, resourceType *ResourceType) (*UsedResourceConfig, error) {
	var resourceCacheID int
	if resourceConfig.CreatedByResourceCache != nil {
		createdByResourceCache, err := resourceConfig.CreatedByResourceCache.FindOrCreateForResourceType(tx, resourceType)
		if err != nil {
			return nil, err
		}

		resourceCacheID = createdByResourceCache.ID
	}
	return resourceConfig.findOrCreate(tx, "resource_type_id", resourceType.ID, resourceCacheID)
}

func (resourceConfig ResourceConfig) findOrCreate(tx Tx, forColumnName string, forColumnID int, resourceCacheID int) (*UsedResourceConfig, error) {
	urc := &UsedResourceConfig{}

	var parentID int
	var parentColumnName string
	if resourceConfig.CreatedByResourceCache != nil {
		parentColumnName = "resource_cache_id"
		parentID = resourceCacheID
	}

	if resourceConfig.CreatedByBaseResourceType != nil {
		parentColumnName = "base_resource_type_id"

		var err error
		urc.CreatedByBaseResourceType, err = resourceConfig.CreatedByBaseResourceType.FindOrCreate(tx)
		if err != nil {
			return nil, err
		}

		parentID = urc.CreatedByBaseResourceType.ID
	}

	id, found, err := resourceConfig.findWithParentID(tx, parentColumnName, parentID)
	if err != nil {
		return nil, err
	}

	if !found {
		err := psql.Insert("resource_configs").
			Columns(
				parentColumnName,
				"source_hash",
			).
			Values(
				parentID,
				resourceConfig.sourceHash(),
			).
			Suffix("RETURNING id").
			RunWith(tx).
			QueryRow().
			Scan(&id)
		if err != nil {
			if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "unique_violation" {
				return nil, ErrResourceConfigAlreadyExists
			}

			if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "foreign_key_violation" {
				return nil, ErrResourceConfigParentDisappeared
			}

			return nil, err
		}
	}

	urc.ID = id

	_, err = psql.Insert("resource_config_uses").
		Columns(
			"resource_config_id",
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
			return nil, ErrResourceConfigDisappeared
		}

		return nil, err
	}

	return urc, nil
}

func (resourceConfig ResourceConfig) findWithParentID(tx Tx, parentColumnName string, parentID int) (int, bool, error) {
	var id int
	err := psql.Select("id").From("resource_configs").Where(sq.Eq{
		parentColumnName: parentID,
		"source_hash":    resourceConfig.sourceHash(),
	}).RunWith(tx).QueryRow().Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, false, nil
		}

		return 0, false, err
	}

	return id, true, nil
}

func (config ResourceConfig) sourceHash() string {
	j, _ := json.Marshal(config.Source)
	return string(j) // TODO: actually hash
}
