package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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
	CheckError() error
	DefaultSpace() atc.Space
	CreatedByResourceCache() UsedResourceCache
	CreatedByBaseResourceType() *UsedBaseResourceType
	OriginBaseResourceType() *UsedBaseResourceType

	AcquireResourceConfigCheckingLockWithIntervalCheck(
		logger lager.Logger,
		interval time.Duration,
		immediate bool,
	) (lock.Lock, bool, error)

	SaveUncheckedVersion(space atc.Space, version atc.Version, metadata ResourceConfigMetadataFields) (bool, error)
	FindUncheckedVersion(atc.Space, atc.Version) (ResourceConfigVersion, bool, error)
	FindVersion(atc.Space, atc.Version) (ResourceConfigVersion, bool, error)
	LatestVersions() ([]ResourceConfigVersion, error)

	SaveDefaultSpace(atc.Space) error
	SavePartialVersion(atc.Space, atc.Version, atc.Metadata) error
	SaveSpace(atc.Space) error
	SaveSpaceLatestVersion(atc.Space, atc.Version) error
	FinishSavingVersions() error

	SetCheckError(error) error
}

type resourceConfig struct {
	id                        int
	checkError                error
	defaultSpace              atc.Space
	createdByResourceCache    UsedResourceCache
	createdByBaseResourceType *UsedBaseResourceType
	lockFactory               lock.LockFactory
	conn                      Conn
}

func (r *resourceConfig) ID() int                                   { return r.id }
func (r *resourceConfig) CheckError() error                         { return r.checkError }
func (r *resourceConfig) DefaultSpace() atc.Space                   { return r.defaultSpace }
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
		logger.Debug("failed-to-update-interval", lager.Data{
			"interval":  interval,
			"immediate": immediate,
		})

		lockErr := lock.Release()
		if lockErr != nil {
			logger.Fatal("failed-to-release-lock", lockErr)
		}
		return nil, false, nil
	}

	return lock, true, nil
}

func (r *resourceConfig) LatestVersions() ([]ResourceConfigVersion, error) {
	rows, err := resourceConfigVersionQuery.
		Where(sq.Eq{
			"s.resource_config_id": r.id,
		}).
		Where(sq.Expr("s.latest_resource_version_id = v.id")).
		RunWith(r.conn).
		Query()
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	versions := []ResourceConfigVersion{}
	for rows.Next() {
		rcv := &resourceConfigVersion{
			conn:           r.conn,
			resourceConfig: r,
		}

		err := scanResourceConfigVersion(rcv, rows)
		if err != nil {
			return nil, err
		}

		versions = append(versions, rcv)
	}

	return versions, nil
}

// SaveUncheckedVersion is used by the "get" and "put" step to find or create of a
// resource config version. We want to do an upsert because there will be cases
// where resource config versions can become outdated while the versions
// associated to it are still valid. This will be special case where we save
// the version with a check order of 0 in order to avoid using this version
// until we do a proper check. Note that this method will not bump the cache
// index for the pipeline because we want to ignore these versions until the
// check orders get updated. The bumping of the index will be done in
// SaveOutput for the put step.
func (r *resourceConfig) SaveUncheckedVersion(space atc.Space, version atc.Version, metadata ResourceConfigMetadataFields) (bool, error) {
	tx, err := r.conn.Begin()
	if err != nil {
		return false, err
	}

	defer Rollback(tx)

	newVersion, err := saveResourceVersion(tx, r, space, version, metadata.ToATCMetadata(), false)
	if err != nil {
		return false, err
	}

	return newVersion, tx.Commit()
}

func (r *resourceConfig) FindUncheckedVersion(space atc.Space, version atc.Version) (ResourceConfigVersion, bool, error) {
	return r.findVersion(space, version, uncheckedResourceConfigVersionQuery)
}

func (r *resourceConfig) FindVersion(space atc.Space, version atc.Version) (ResourceConfigVersion, bool, error) {
	return r.findVersion(space, version, resourceConfigVersionQuery)
}

func (r *resourceConfig) SetCheckError(cause error) error {
	var err error

	if cause == nil {
		_, err = psql.Update("resource_configs").
			Set("check_error", nil).
			Where(sq.Eq{"id": r.id}).
			RunWith(r.conn).
			Exec()
	} else {
		_, err = psql.Update("resource_configs").
			Set("check_error", cause.Error()).
			Where(sq.Eq{"id": r.id}).
			RunWith(r.conn).
			Exec()
	}

	return err
}

// SavePartialVersion stores a version into the db for a resource config
// Each version will also have its check order field updated incrementally.
//
// In the case of a check resource from an older version, the versions
// that already exist in the DB will be re-ordered using
// incrementCheckOrderWhenNewerVersion to input the correct check order
func (r *resourceConfig) SavePartialVersion(space atc.Space, version atc.Version, metadata atc.Metadata) error {
	tx, err := r.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	_, err = saveResourceVersion(tx, r, space, version, metadata, true)
	if err != nil {
		return err
	}

	versionJSON, err := json.Marshal(version)
	if err != nil {
		return err
	}

	err = incrementCheckOrder(tx, r, space, string(versionJSON))
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *resourceConfig) SaveDefaultSpace(defaultSpace atc.Space) error {
	_, err := psql.Update("resource_configs").
		Set("default_space", defaultSpace).
		Where(sq.Eq{"id": r.id}).
		RunWith(r.conn).
		Exec()
	return err
}

func (r *resourceConfig) SaveSpace(space atc.Space) error {
	_, err := psql.Insert("spaces").
		Columns("resource_config_id", "name").
		Values(r.id, space).
		Suffix("ON CONFLICT DO NOTHING").
		RunWith(r.conn).
		Exec()
	return err
}

func (r *resourceConfig) SaveSpaceLatestVersion(space atc.Space, version atc.Version) error {
	versionBlob, err := json.Marshal(version)
	if err != nil {
		return err
	}

	latestIdQuery := fmt.Sprintf("( SELECT id FROM resource_versions WHERE version_md5 = md5('%s') )", versionBlob)

	_, err = psql.Update("spaces").
		Set("latest_resource_version_id", sq.Expr(latestIdQuery)).
		Where(sq.Eq{
			"resource_config_id": r.id,
			"name":               space,
		}).
		RunWith(r.conn).
		Exec()

	return err
}

func (r *resourceConfig) FinishSavingVersions() error {
	tx, err := r.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	_, err = tx.Exec(`UPDATE resource_versions
		SET partial = false
		FROM spaces
		WHERE spaces.resource_config_id = $1
		AND spaces.id = resource_versions.space_id;`, r.id)

	err = bumpCacheIndexForPipelinesUsingResourceConfig(tx, r.id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// increment the check order if the version's check order is less than the
// current max. This will fix the case of a check from an old version causing
// the desired order to change; existing versions will be re-ordered since
// we add them in the desired order.
func incrementCheckOrder(tx Tx, r ResourceConfig, space atc.Space, version string) error {
	_, err := tx.Exec(`
		WITH max_checkorder AS (
			SELECT max(v.check_order) co
			FROM resource_versions v, spaces s
			WHERE s.resource_config_id = $1
			AND s.name = $3
			AND s.id = v.space_id
		)

		UPDATE resource_versions
		SET check_order = mc.co + 1
		FROM max_checkorder mc, spaces s
		WHERE space_id = s.id
		AND s.resource_config_id = $1
		AND s.name = $3
		AND version = $2
		AND check_order <= mc.co;`, r.ID(), version, space)
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

func (r *resourceConfig) findVersion(space atc.Space, version atc.Version, query sq.SelectBuilder) (ResourceConfigVersion, bool, error) {
	rcv := &resourceConfigVersion{
		resourceConfig: r,
		conn:           r.conn,
	}

	versionByte, err := json.Marshal(version)
	if err != nil {
		return nil, false, err
	}

	row := query.
		Where(sq.Eq{
			"s.resource_config_id": r.id,
			"s.name":               space,
		}).
		Where(sq.Expr("s.id = v.space_id")).
		Where(sq.Expr(fmt.Sprintf("v.version_md5 = md5('%s')", versionByte))).
		RunWith(r.conn).
		QueryRow()

	err = scanResourceConfigVersion(rcv, row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	return rcv, true, nil
}

func saveResourceConfigVersion(tx Tx, r ResourceConfig, version atc.Version, metadata ResourceConfigMetadataFields) (bool, error) {
	versionJSON, err := json.Marshal(version)
	if err != nil {
		return false, err
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return false, err
	}

	var checkOrder int
	err = tx.QueryRow(`
		INSERT INTO resource_config_versions (resource_config_id, version, version_md5, metadata)
		SELECT $1, $2, md5($3), $4
		ON CONFLICT (resource_config_id, version_md5) DO UPDATE SET metadata = $4
		RETURNING check_order
		`, r.ID(), string(versionJSON), string(versionJSON), string(metadataJSON)).Scan(&checkOrder)
	if err != nil {
		return false, err
	}

	return checkOrder == 0, nil
}

func saveResourceVersion(tx Tx, r ResourceConfig, space atc.Space, version atc.Version, metadata atc.Metadata, partial bool) (bool, error) {
	versionJSON, err := json.Marshal(version)
	if err != nil {
		return false, err
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return false, err
	}

	var checkOrder int
	err = tx.QueryRow(`
		INSERT INTO resource_versions (space_id, version, version_md5, metadata, partial)
		SELECT s.id, $2, md5($3), $4, $6
		FROM spaces s WHERE s.resource_config_id = $1 AND s.name = $5
		ON CONFLICT (space_id, version_md5) DO UPDATE SET metadata = $4
		RETURNING check_order
		`, r.ID(), string(versionJSON), string(versionJSON), string(metadataJSON), string(space), partial).Scan(&checkOrder)
	if err != nil {
		return false, err
	}

	return checkOrder == 0, nil
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

	id, checkError, defaultSpace, found, err := r.findWithParentID(tx, parentColumnName, parentID)
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
	rc.checkError = checkError
	rc.defaultSpace = defaultSpace

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

	id, checkError, defaultSpace, found, err := r.findWithParentID(tx, parentColumnName, parentID)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	rc.id = id
	rc.checkError = checkError
	rc.defaultSpace = defaultSpace

	return rc, true, nil
}

func (r *ResourceConfigDescriptor) findWithParentID(tx Tx, parentColumnName string, parentID int) (int, error, atc.Space, bool, error) {
	var (
		id         int
		checkError sql.NullString
		defSpace   sql.NullString
	)

	err := psql.Select("id, check_error, default_space").
		From("resource_configs").
		Where(sq.Eq{
			parentColumnName: parentID,
			"source_hash":    mapHash(r.Source),
		}).
		Suffix("FOR SHARE").
		RunWith(tx).
		QueryRow().
		Scan(&id, &checkError, &defSpace)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil, "", false, nil
		}

		return 0, nil, "", false, err
	}

	var chkErr error
	if checkError.Valid {
		chkErr = errors.New(checkError.String)
	}

	var defaultSpace atc.Space
	if defSpace.Valid {
		defaultSpace = atc.Space(defSpace.String)
	}

	return id, chkErr, defaultSpace, true, nil
}

func bumpCacheIndexForPipelinesUsingResourceConfig(tx Tx, rcID int) error {
	_, err := tx.Exec(`
		UPDATE pipelines p
		SET cache_index = cache_index + 1
		FROM resources r
		WHERE r.pipeline_id = p.id
		AND r.resource_config_id = $1
	`, rcID)
	if err != nil {
		return err
	}

	return nil
}
