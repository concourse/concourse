package db

import (
	"errors"
	"fmt"
	"time"

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

//go:generate counterfeiter . ResourceConfig

type ResourceConfig interface {
	ID() int
	LastReferenced() time.Time
	CreatedByResourceCache() ResourceCache
	CreatedByBaseResourceType() *UsedBaseResourceType

	OriginBaseResourceType() *UsedBaseResourceType

	FindOrCreateScope(Resource) (ResourceConfigScope, error)
}

// ResourceConfig represents a resource type and config source.
//
// Resources in a pipeline, resource types in a pipeline, and `image_resource`
// fields in a task all result in a reference to a ResourceConfig.
//
// ResourceConfigs are garbage-collected by gc.ResourceConfigCollector.
type resourceConfig struct {
	id                        int
	lastReferenced            time.Time
	createdByResourceCache    ResourceCache
	createdByBaseResourceType *UsedBaseResourceType
	lockFactory               lock.LockFactory
	conn                      Conn
}

func (r *resourceConfig) ID() int {
	return r.id
}

func (r *resourceConfig) LastReferenced() time.Time {
	return r.lastReferenced
}

func (r *resourceConfig) CreatedByResourceCache() ResourceCache {
	return r.createdByResourceCache
}

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

func (r *resourceConfig) updateLastReferenced(tx Tx) error {
	return psql.Update("resource_configs").
		Set("last_referenced", sq.Expr("now()")).
		Where(sq.Eq{"id": r.id}).
		Suffix("RETURNING last_referenced").
		RunWith(tx).
		QueryRow().
		Scan(&r.lastReferenced)
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

	rows, err := psql.Select("id").
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
		err = rows.Scan(&scopeID)
		if err != nil {
			return nil, err
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
		conn:           conn,
		lockFactory:    lockFactory,
	}, nil
}
