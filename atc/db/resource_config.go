package db

import (
	"database/sql"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/lock"
)

type BaseResourceTypeNotFoundError struct {
	Name string
}

func (e BaseResourceTypeNotFoundError) Error() string {
	return fmt.Sprintf("base resource type not found: %s", e.Name)
}

var ErrResourceConfigAlreadyExists = errors.New("resource config already exists")
var ErrResourceConfigDisappeared = errors.New("resource config disappeared")
var ErrResourceConfigParentDisappeared = errors.New("resource config parent disappeared")
var ErrResourceConfigHasNoType = errors.New("resource config has no type")

// ResourceConfig represents a resource type and config source.
//
// Resources in a pipeline, resource types in a pipeline, and `image_resource`
// fields in a task all result in a reference to a ResourceConfig.
//
// ResourceConfigs are garbage-collected by gc.ResourceConfigCollector.
type ResourceConfigDescriptor struct {
	// A resource type provided by a resource.
	CreatedByResourceCache *ResourceCacheDescriptor

	// A resource type provided by a worker.
	CreatedByBaseResourceType *BaseResourceType

	// The resource's source configuration.
	Source atc.Source
}

//go:generate counterfeiter . ResourceConfig

type ResourceConfig interface {
	ID() int
	CreatedByResourceCache() UsedResourceCache
	CreatedByBaseResourceType() *UsedBaseResourceType
	OriginBaseResourceType() *UsedBaseResourceType

	FindOrCreateScope(Resource) (ResourceConfigScope, error)
}

type resourceConfig struct {
	id                        int
	createdByResourceCache    UsedResourceCache
	createdByBaseResourceType *UsedBaseResourceType
	lockFactory               lock.LockFactory
	conn                      Conn
}

func (r *resourceConfig) ID() int                                   { return r.id }
func (r *resourceConfig) CreatedByResourceCache() UsedResourceCache { return r.createdByResourceCache }
func (r *resourceConfig) CreatedByBaseResourceType() *UsedBaseResourceType {
	return r.createdByBaseResourceType
}

func (r *resourceConfig) OriginBaseResourceType() *UsedBaseResourceType {
	if r.createdByBaseResourceType != nil {
		return r.createdByBaseResourceType
	}
	return r.createdByResourceCache.ResourceConfig().OriginBaseResourceType()
}

func (r *resourceConfig) FindOrCreateScope(resource Resource) (ResourceConfigScope, error) {
	tx, err := r.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	scope, err := findOrCreateResourceConfigScope(
		tx,
		r.conn,
		r.lockFactory,
		r,
		resource,
	)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return scope, nil
}

func (r *ResourceConfigDescriptor) findOrCreate(tx Tx, lockFactory lock.LockFactory, conn Conn) (ResourceConfig, error) {
	rc := &resourceConfig{
		lockFactory: lockFactory,
		conn:        conn,
	}

	var parentID int
	var parentColumnName string
	if r.CreatedByResourceCache != nil {
		parentColumnName = "resource_cache_id"

		resourceCache, err := r.CreatedByResourceCache.findOrCreate(tx, lockFactory, conn)
		if err != nil {
			return nil, err
		}

		parentID = resourceCache.ID()

		rc.createdByResourceCache = resourceCache
	}

	if r.CreatedByBaseResourceType != nil {
		parentColumnName = "base_resource_type_id"

		var err error
		var found bool
		rc.createdByBaseResourceType, found, err = r.CreatedByBaseResourceType.Find(tx)
		if err != nil {
			return nil, err
		}

		if !found {
			return nil, BaseResourceTypeNotFoundError{Name: r.CreatedByBaseResourceType.Name}
		}

		parentID = rc.CreatedByBaseResourceType().ID
	}

	id, found, err := r.findWithParentID(tx, parentColumnName, parentID)
	if err != nil {
		return nil, err
	}

	if !found {
		hash := mapHash(r.Source)

		var err error
		err = psql.Insert("resource_configs").
			Columns(
				parentColumnName,
				"source_hash",
			).
			Values(
				parentID,
				hash,
			).
			Suffix(`
				ON CONFLICT (`+parentColumnName+`, source_hash) DO UPDATE SET
					`+parentColumnName+` = ?,
					source_hash = ?
				RETURNING id
			`, parentID, hash).
			RunWith(tx).
			QueryRow().
			Scan(&id)

		if err != nil {
			return nil, err
		}
	}

	rc.id = id

	return rc, nil
}

func (r *ResourceConfigDescriptor) find(tx Tx, lockFactory lock.LockFactory, conn Conn) (ResourceConfig, bool, error) {
	rc := &resourceConfig{
		lockFactory: lockFactory,
		conn:        conn,
	}

	var parentID int
	var parentColumnName string
	if r.CreatedByResourceCache != nil {
		parentColumnName = "resource_cache_id"

		resourceCache, found, err := r.CreatedByResourceCache.find(tx, lockFactory, conn)
		if err != nil {
			return nil, false, err
		}

		if !found {
			return nil, false, nil
		}

		parentID = resourceCache.ID()

		rc.createdByResourceCache = resourceCache
	}

	if r.CreatedByBaseResourceType != nil {
		parentColumnName = "base_resource_type_id"

		var err error
		var found bool
		rc.createdByBaseResourceType, found, err = r.CreatedByBaseResourceType.Find(tx)
		if err != nil {
			return nil, false, err
		}

		if !found {
			return nil, false, nil
		}

		parentID = rc.createdByBaseResourceType.ID
	}

	id, found, err := r.findWithParentID(tx, parentColumnName, parentID)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	rc.id = id

	return rc, true, nil
}

func (r *ResourceConfigDescriptor) findWithParentID(tx Tx, parentColumnName string, parentID int) (int, bool, error) {
	var id int
	var whereClause sq.Eq

	err := psql.Select("id").
		From("resource_configs").
		Where(sq.Eq{
			parentColumnName: parentID,
			"source_hash":    mapHash(r.Source),
		}).
		Where(whereClause).
		Suffix("FOR SHARE").
		RunWith(tx).
		QueryRow().
		Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, false, nil
		}

		return 0, false, err
	}

	return id, true, nil
}

func findOrCreateResourceConfigScope(
	tx Tx,
	conn Conn,
	lockFactory lock.LockFactory,
	resourceConfig ResourceConfig,
	resource Resource,
) (ResourceConfigScope, error) {
	var uniqueResource Resource
	var resourceID *int

	if resource != nil {
		var unique bool
		if !atc.EnableGlobalResources {
			unique = true
		} else {
			if brt := resourceConfig.CreatedByBaseResourceType(); brt != nil {
				unique = brt.UniqueVersionHistory
			}
		}

		if unique {
			id := resource.ID()

			resourceID = &id
			uniqueResource = resource
		}
	}

	var scopeID int
	var checkErr error

	rows, err := psql.Select("id, check_error").
		From("resource_config_scopes").
		Where(sq.Eq{
			"resource_id":        resourceID,
			"resource_config_id": resourceConfig.ID(),
		}).
		RunWith(tx).
		Query()
	if err != nil {
		return nil, err
	}

	if rows.Next() {
		var checkErrBlob sql.NullString

		err = rows.Scan(&scopeID, &checkErrBlob)
		if err != nil {
			return nil, err
		}

		if checkErrBlob.Valid {
			checkErr = errors.New(checkErrBlob.String)
		}

		err = rows.Close()
		if err != nil {
			return nil, err
		}
	} else if uniqueResource != nil {
		// delete outdated scopes for resource
		_, err := psql.Delete("resource_config_scopes").
			Where(sq.And{
				sq.Eq{
					"resource_id": resource.ID(),
				},
			}).
			RunWith(tx).
			Exec()
		if err != nil {
			return nil, err
		}

		err = psql.Insert("resource_config_scopes").
			Columns("resource_id", "resource_config_id").
			Values(resource.ID(), resourceConfig.ID()).
			Suffix(`
				ON CONFLICT (resource_id, resource_config_id) WHERE resource_id IS NOT NULL DO UPDATE SET
					resource_id = ?,
					resource_config_id = ?
				RETURNING id
			`, resource.ID(), resourceConfig.ID()).
			RunWith(tx).
			QueryRow().
			Scan(&scopeID)
		if err != nil {
			return nil, err
		}
	} else {
		err = psql.Insert("resource_config_scopes").
			Columns("resource_id", "resource_config_id").
			Values(nil, resourceConfig.ID()).
			Suffix(`
				ON CONFLICT (resource_config_id) WHERE resource_id IS NULL DO UPDATE SET
					resource_config_id = ?
				RETURNING id
			`, resourceConfig.ID()).
			RunWith(tx).
			QueryRow().
			Scan(&scopeID)
		if err != nil {
			return nil, err
		}
	}

	return &resourceConfigScope{
		id:             scopeID,
		resource:       uniqueResource,
		resourceConfig: resourceConfig,
		checkError:     checkErr,
		conn:           conn,
		lockFactory:    lockFactory,
	}, nil
}
