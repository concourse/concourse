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

type ResourceConfig struct {
	CreatedByResourceCache    *ResourceCache
	CreatedByBaseResourceType *BaseResourceType

	Source atc.Source
	Params atc.Params
}

type UsedResourceConfig struct {
	ID int

	CreatedByResourceCache    *UsedResourceCache
	CreatedByBaseResourceType *UsedBaseResourceType
}

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
				"params_hash",
			).
			Values(
				parentID,
				resourceConfig.hash(resourceConfig.Source),
				resourceConfig.hash(resourceConfig.Params),
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
		"source_hash":    resourceConfig.hash(resourceConfig.Source),
		"params_hash":    resourceConfig.hash(resourceConfig.Params),
		parentColumnName: parentID,
	}).RunWith(tx).QueryRow().Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, false, nil
		}

		return 0, false, err
	}

	return id, true, nil
}

func (ResourceConfig) hash(prop interface{}) string {
	j, _ := json.Marshal(prop)
	return string(j) // TODO: actually hash
}
