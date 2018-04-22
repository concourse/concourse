package db

import (
	"database/sql"
	"errors"

	"code.cloudfoundry.org/lager"

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
//
// XXX: don't call this Used
type UsedResourceConfig struct {
	ID                        int
	CreatedByResourceCache    *UsedResourceCache
	CreatedByBaseResourceType *UsedBaseResourceType
}

func (resourceConfig *UsedResourceConfig) OriginBaseResourceType() *UsedBaseResourceType {
	if resourceConfig.CreatedByBaseResourceType != nil {
		return resourceConfig.CreatedByBaseResourceType
	}
	return resourceConfig.CreatedByResourceCache.ResourceConfig.OriginBaseResourceType()
}

func (resourceConfig ResourceConfig) findOrCreate(logger lager.Logger, tx Tx) (*UsedResourceConfig, error) {
	urc := &UsedResourceConfig{}

	var parentID int
	var parentColumnName string
	if resourceConfig.CreatedByResourceCache != nil {
		parentColumnName = "resource_cache_id"

		resourceCache, err := resourceConfig.CreatedByResourceCache.findOrCreate(logger, tx)
		if err != nil {
			return nil, err
		}

		parentID = resourceCache.ID

		urc.CreatedByResourceCache = resourceCache
	}

	if resourceConfig.CreatedByBaseResourceType != nil {
		parentColumnName = "base_resource_type_id"

		var err error
		var found bool
		urc.CreatedByBaseResourceType, found, err = resourceConfig.CreatedByBaseResourceType.Find(tx)
		if err != nil {
			return nil, err
		}

		if !found {
			return nil, ResourceTypeNotFoundError{Name: resourceConfig.CreatedByBaseResourceType.Name}
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
				mapHash(resourceConfig.Source),
			).
			Suffix("RETURNING id").
			RunWith(tx).
			QueryRow().
			Scan(&id)
		if err != nil {
			if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == pqUniqueViolationErrCode {
				return nil, ErrSafeRetryFindOrCreate
			}

			if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == pqFKeyViolationErrCode {
				return nil, ErrSafeRetryFindOrCreate
			}

			return nil, err
		}
	}

	urc.ID = id

	return urc, nil
}

func (resourceConfig ResourceConfig) Find(tx Tx) (*UsedResourceConfig, bool, error) {
	urc := &UsedResourceConfig{}

	var parentID int
	var parentColumnName string
	if resourceConfig.CreatedByResourceCache != nil {
		parentColumnName = "resource_cache_id"

		resourceCache, found, err := resourceConfig.CreatedByResourceCache.Find(tx)
		if err != nil {
			return nil, false, err
		}

		if !found {
			return nil, false, nil
		}

		parentID = resourceCache.ID

		urc.CreatedByResourceCache = resourceCache
	}

	if resourceConfig.CreatedByBaseResourceType != nil {
		parentColumnName = "base_resource_type_id"

		var err error
		var found bool
		urc.CreatedByBaseResourceType, found, err = resourceConfig.CreatedByBaseResourceType.Find(tx)
		if err != nil {
			return nil, false, err
		}

		if !found {
			return nil, false, nil
		}

		parentID = urc.CreatedByBaseResourceType.ID
	}

	id, found, err := resourceConfig.findWithParentID(tx, parentColumnName, parentID)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	urc.ID = id

	return urc, true, nil
}

func (resourceConfig ResourceConfig) findWithParentID(tx Tx, parentColumnName string, parentID int) (int, bool, error) {
	var id int
	err := psql.Select("id").From("resource_configs").Where(sq.Eq{
		parentColumnName: parentID,
		"source_hash":    mapHash(resourceConfig.Source),
	}).Suffix("FOR SHARE").RunWith(tx).QueryRow().Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, false, nil
		}

		return 0, false, err
	}

	return id, true, nil
}
