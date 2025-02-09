package db

import (
	"database/sql"
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

//counterfeiter:generate . ResourceConfig
type ResourceConfig interface {
	ID() int
	LastReferenced() time.Time
	CreatedByResourceCache() ResourceCache
	CreatedByBaseResourceType() *UsedBaseResourceType

	OriginBaseResourceType() *UsedBaseResourceType

	FindOrCreateScope(resourceID *int) (ResourceConfigScope, error)
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
	conn                      DbConn
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

func (r *resourceConfig) FindOrCreateScope(resourceID *int) (ResourceConfigScope, error) {
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
		resourceID,
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

func findOrCreateResourceConfigScope(
	tx Tx,
	conn DbConn,
	lockFactory lock.LockFactory,
	resourceConfig ResourceConfig,
	resourceID *int,
) (ResourceConfigScope, error) {
	var uniqueResourceID *int
	if resourceID != nil {
		var unique bool
		if !atc.EnableGlobalResources {
			unique = true
		} else {
			if brt := resourceConfig.CreatedByBaseResourceType(); brt != nil {
				unique = brt.UniqueVersionHistory
			}
		}

		if unique {
			uniqueResourceID = resourceID
		}
	}

	var scopeID int
	var rows *sql.Rows
	var err error
	if uniqueResourceID == nil {
		rows, err = psql.Select("id").
			From("resource_config_scopes").
			Where(sq.Eq{
				"resource_config_id": resourceConfig.ID(),
			}).
			Where(sq.Expr("resource_id IS NULL")).
			RunWith(tx).
			Query()
	} else {
		rows, err = psql.Select("id").
			From("resource_config_scopes").
			Where(sq.Eq{
				"resource_id":        resourceID,
				"resource_config_id": resourceConfig.ID(),
			}).
			RunWith(tx).
			Query()
	}
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
	} else if uniqueResourceID != nil {
		// This `SELECT ... FOR UPDATE` on the resource is just to avoid a
		// deadlock, which occurs when concurrently setting a pipeline and
		// running FindOrCreateScope on the resource that's being updated in
		// the pipeline. Specifically, it happens with the following "DELETE
		// FROM resource_config_scopes" query - this deletes the old resource
		// config scope, which in turn triggers an "ON DELETE SET NULL" in the
		// resource. However, there's some implicit lock that's acquired when
		// setting the pipeline on the resource_config_scope, and without this
		// dummy query, the locks are acquired in a bad order wrt one another:
		//
		// DELETE FROM resource_config_scopes:
		//    1. Lock resource_config_scopes
		//    2. Lock resource
		//
		// INSERT INTO resources (occurs when setting the pipeline):
		//    1. Lock resource
		//    2. Lock resource_config_scope
		//
		// Thus, forcing the DELETE FROM resource_config_scopes query to
		// acquire a lock on the affected resource fixes this order (first
		// resource, then resource_config_scope) to avoid a cycle.
		_, err := psql.Select("1").
			From("resources").
			Where(sq.Eq{
				"id": *uniqueResourceID,
			}).
			Suffix("FOR UPDATE").
			RunWith(tx).
			Exec()
		if err != nil {
			return nil, err
		}

		// delete outdated scopes for resource
		_, err = psql.Delete("resource_config_scopes").
			Where(sq.Eq{
				"resource_id": *uniqueResourceID,
			}).
			Where(sq.NotEq{
				"resource_config_id": resourceConfig.ID(),
			}).
			RunWith(tx).
			Exec()
		if err != nil {
			return nil, err
		}

		err = psql.Insert("resource_config_scopes").
			Columns("resource_id", "resource_config_id").
			Values(*resourceID, resourceConfig.ID()).
			Suffix(`
				ON CONFLICT (resource_id, resource_config_id) WHERE resource_id IS NOT NULL DO UPDATE SET
					resource_id = ?,
					resource_config_id = ?
				RETURNING id
			`, *uniqueResourceID, resourceConfig.ID()).
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
		resourceID:     uniqueResourceID,
		resourceConfig: resourceConfig,
		conn:           conn,
		lockFactory:    lockFactory,
	}, nil
}
