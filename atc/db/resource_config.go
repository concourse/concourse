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

//counterfeiter:generate . ResourceConfig
type ResourceConfig interface {
	ID() int
	LastReferenced() time.Time
	CreatedByResourceCache() UsedResourceCache
	CreatedByBaseResourceType() *UsedBaseResourceType

	OriginBaseResourceType() *UsedBaseResourceType

	FindOrCreateScope(Resource) (ResourceConfigScope, error)
}

type resourceConfig struct {
	id                        int
	lastReferenced            time.Time
	createdByResourceCache    UsedResourceCache
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

func (r *resourceConfig) CreatedByResourceCache() UsedResourceCache {
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

func (r *ResourceConfigDescriptor) findOrCreate(tx Tx, lockFactory lock.LockFactory, conn Conn, updateLastReferenced bool) (*resourceConfig, error) {
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

	var found bool
	var err error
	if updateLastReferenced {
		found, err = r.updateLastReferenced(tx, rc, parentColumnName, parentID)
	} else {
		found, err = r.findWithParentID(tx, rc, parentColumnName, parentID)
	}
	if err != nil {
		return nil, err
	}

	if !found {
		hash := mapHash(r.Source)

		valueMap := map[string]interface{}{
			parentColumnName: parentID,
			"source_hash":    hash,
		}
		if updateLastReferenced {
			valueMap["last_referenced"] = sq.Expr("now()")
		}
		var updateLastReferencedStr string
		if updateLastReferenced {
			updateLastReferencedStr = `, last_referenced = now()`
		}
		err := psql.Insert("resource_configs").
			SetMap(valueMap).
			Suffix(`
				ON CONFLICT (`+parentColumnName+`, source_hash) DO UPDATE SET
					`+parentColumnName+` = ?,
					source_hash = ?`+
				updateLastReferencedStr+`
				RETURNING id, last_referenced
			`, parentID, hash).
			RunWith(tx).
			QueryRow().
			Scan(&rc.id, &rc.lastReferenced)
		if err != nil {
			return nil, err
		}
	}

	return rc, nil
}

func (r *ResourceConfigDescriptor) findWithParentID(tx Tx, rc *resourceConfig, parentColumnName string, parentID int) (bool, error) {
	err := psql.Select("id", "last_referenced").
		From("resource_configs").
		Where(sq.Eq{
			parentColumnName: parentID,
			"source_hash":    mapHash(r.Source),
		}).
		Suffix("FOR SHARE").
		RunWith(tx).
		QueryRow().
		Scan(&rc.id, &rc.lastReferenced)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

// updateLastReferenced is called extremely frequently, which generates a lot of
// slow queries. However, last_referenced is only used for gc, it doesn't have to
// be super precise. Based on that, let's query first, and if last_update is within
// 1 minutes, then skip current update.
func (r *ResourceConfigDescriptor) updateLastReferenced(tx Tx, rc *resourceConfig, parentColumnName string, parentID int) (bool, error) {
	err := psql.Select("id", "last_referenced").
		From("resource_configs").
		Where(sq.Eq{
			parentColumnName: parentID,
			"source_hash":    mapHash(r.Source),
		}).
		RunWith(tx).
		QueryRow().
		Scan(&rc.id, &rc.lastReferenced)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}

		return false, err
	}

	if rc.lastReferenced.Add(time.Minute).After(time.Now()) {
		return true, nil
	}

	err = psql.Update("resource_configs").
		Set("last_referenced", sq.Expr("now()")).
		Where(sq.Eq{
			"id": rc.id,
		}).
		Suffix("RETURNING last_referenced").
		RunWith(tx).
		QueryRow().
		Scan(&rc.lastReferenced)
	if err != nil {
		return false, err
	}

	return true, nil
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
				"id": uniqueResource.ID(),
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
				"resource_id": resource.ID(),
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
