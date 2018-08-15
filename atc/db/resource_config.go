package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"code.cloudfoundry.org/lager"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/lock"
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
	AcquireResourceConfigCheckingLockWithIntervalCheck(
		logger lager.Logger,
		interval time.Duration,
		immediate bool,
	) (lock.Lock, bool, error)

	GetLatestVersion() (ResourceConfigVersion, bool, error)
	SaveVersions(versions []atc.Version) error
}

type resourceConfig struct {
	id                        int
	createdByResourceCache    UsedResourceCache
	createdByBaseResourceType *UsedBaseResourceType
	lockFactory               lock.LockFactory
	conn                      Conn
}

func (r *resourceConfig) ID() int {
	return r.id
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

func (r *resourceConfig) AcquireResourceConfigCheckingLockWithIntervalCheck(
	logger lager.Logger,
	interval time.Duration,
	immediate bool,
) (lock.Lock, bool, error) {
	lock, acquired, err := r.lockFactory.Acquire(
		logger,
		lock.NewResourceConfigCheckingLockID(r.id),
	)
	if err != nil {
		return nil, false, err
	}

	if !acquired {
		return nil, false, nil
	}

	intervalUpdated, err := r.checkIfResourceConfigIntervalUpdated(interval, immediate)
	if err != nil {
		lockErr := lock.Release()
		if lockErr != nil {
			logger.Fatal("failed-to-release-lock", lockErr)
		}
		return nil, false, err
	}

	if !intervalUpdated {
		lockErr := lock.Release()
		if lockErr != nil {
			logger.Fatal("failed-to-release-lock", lockErr)
		}
		return nil, false, nil
	}

	return lock, true, nil
}

func (r *resourceConfig) GetLatestVersion() (ResourceConfigVersion, bool, error) {
	var version, metadata string

	rcv := &resourceConfigVersion{
		resourceConfigID: r.id,
	}

	err := psql.Select("v.id, v.version, v.metadata, v.check_order").
		From("resource_config_versions v").
		Where(sq.Eq{
			"v.resource_config_id": r.id,
		}).
		OrderBy("check_order DESC").
		Limit(1).
		RunWith(r.conn).
		QueryRow().
		Scan(&rcv.id, &version, &metadata, &rcv.checkOrder)
	if err != nil {
		if err == sql.ErrNoRows {
			return &resourceConfigVersion{}, false, nil
		}

		return &resourceConfigVersion{}, false, err
	}

	err = json.Unmarshal([]byte(version), &rcv.version)
	if err != nil {
		return &resourceConfigVersion{}, false, err
	}

	err = json.Unmarshal([]byte(metadata), &rcv.metadata)
	if err != nil {
		return &resourceConfigVersion{}, false, err
	}

	return rcv, true, nil
}

func (r *resourceConfig) SaveVersions(versions []atc.Version) error {
	tx, err := r.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	for _, version := range versions {
		rcv := &resourceConfigVersion{
			resourceConfigID: r.id,
			version:          Version(version),
		}

		versionJSON, err := json.Marshal(rcv.version)
		if err != nil {
			return err
		}

		_, err = r.saveResourceConfigVersion(tx, rcv)
		if err != nil {
			return err
		}

		err = r.incrementCheckOrderWhenNewerVersion(tx, string(versionJSON))
		if err != nil {
			return err
		}
	}

	// XXX: IDKKKKKK
	// err = bumpCacheIndex(tx, p.id)
	// if err != nil {
	// 	return err
	// }

	return tx.Commit()
}

func (r *resourceConfig) saveResourceConfigVersion(tx Tx, rcv ResourceConfigVersion) (ResourceConfigVersion, error) {
	versionJSON, err := json.Marshal(rcv.Version())
	if err != nil {
		return nil, err
	}

	metadataJSON, err := json.Marshal(rcv.Metadata())
	if err != nil {
		return nil, err
	}

	var id, resourceConfigID, checkOrder int
	var version, metadata string

	// XXX uniq
	err = tx.QueryRow(`
		INSERT INTO resource_config_versions (resource_config_id, version, metadata)
		SELECT $1, $2, $3
		ON CONFLICT (resource_config_id, version) DO UPDATE SET metadata = $3
		RETURNING id, resource_config_id, check_order, version, metadata
		`, r.ID(), string(versionJSON), string(metadataJSON)).Scan(&id, &resourceConfigID, &checkOrder, &version, &metadata)
	if err != nil {
		return nil, err
	}

	savedRCV := &resourceConfigVersion{
		id:               id,
		resourceConfigID: resourceConfigID,
		checkOrder:       checkOrder,
	}

	err = json.Unmarshal([]byte(version), &savedRCV.version)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal([]byte(metadata), &savedRCV.metadata)
	if err != nil {
		return nil, err
	}

	return savedRCV, nil
}

func (r *resourceConfig) incrementCheckOrderWhenNewerVersion(tx Tx, version string) error {
	_, err := tx.Exec(`
		WITH max_checkorder AS (
			SELECT max(check_order) co
			FROM resource_config_versions
			WHERE resource_config_id = $1
		)

		UPDATE resource_config_versions
		SET check_order = mc.co + 1
		FROM max_checkorder mc
		WHERE resource_config_id = $1
		AND version = $2
		AND check_order <= mc.co;`, r.ID(), version)
	return err
}

func (r *resourceConfig) checkIfResourceConfigIntervalUpdated(
	interval time.Duration,
	immediate bool,
) (bool, error) {
	tx, err := r.conn.Begin()
	if err != nil {
		return false, err
	}

	defer Rollback(tx)

	params := []interface{}{r.id}

	condition := ""
	if !immediate {
		condition = "AND now() - last_checked > ($2 || ' SECONDS')::INTERVAL"
		params = append(params, interval.Seconds())
	}

	updated, err := checkIfRowsUpdated(tx, `
			UPDATE resource_configs
			SET last_checked = now()
			WHERE id = $1
		`+condition, params...)
	if err != nil {
		return false, err
	}

	if !updated {
		return false, nil
	}

	err = tx.Commit()
	if err != nil {
		return false, err
	}

	return true, nil
}

func (r *ResourceConfigDescriptor) findOrCreate(logger lager.Logger, tx Tx, lockFactory lock.LockFactory, conn Conn) (ResourceConfig, error) {
	rc := &resourceConfig{
		lockFactory: lockFactory,
		conn:        conn,
	}

	var parentID int
	var parentColumnName string
	if r.CreatedByResourceCache != nil {
		parentColumnName = "resource_cache_id"

		resourceCache, err := r.CreatedByResourceCache.findOrCreate(logger, tx, lockFactory, conn)
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
			return nil, ResourceTypeNotFoundError{Name: r.CreatedByBaseResourceType.Name}
		}

		parentID = rc.CreatedByBaseResourceType().ID
	}

	id, found, err := r.findWithParentID(tx, parentColumnName, parentID)
	if err != nil {
		return nil, err
	}

	if !found {
		hash := mapHash(r.Source)

		err := psql.Insert("resource_configs").
			Columns(
				parentColumnName,
				"source_hash",
			).
			Values(
				parentID,
				hash,
			).
			Suffix(`
				ON CONFLICT (resource_cache_id, base_resource_type_id, source_hash) DO UPDATE SET
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
	err := psql.Select("id").
		From("resource_configs").
		Where(sq.Eq{
			parentColumnName: parentID,
			"source_hash":    mapHash(r.Source),
		}).
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
