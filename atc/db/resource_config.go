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
	CheckError() error
	DefaultSpace() atc.Space

	FindVersion(atc.Space, atc.Version) (ResourceVersion, bool, error)
	LatestVersions() ([]ResourceVersion, error)

	SaveDefaultSpace(atc.Space) error
	SavePartialVersion(atc.Space, atc.Version, atc.Metadata) error
	SaveSpace(atc.Space) error
	SaveSpaceLatestVersion(atc.Space, atc.Version) error
	FinishSavingVersions() error

	SetCheckError(error) error

	AcquireResourceCheckingLock(
		logger lager.Logger,
	) (lock.Lock, bool, error)

	UpdateLastChecked(
		interval time.Duration,
		immediate bool,
	) (bool, error)

	UpdateLastCheckFinished() (bool, error)
}

type resourceConfig struct {
	id                        int
	createdByResourceCache    UsedResourceCache
	createdByBaseResourceType *UsedBaseResourceType
	checkError                error
	defaultSpace              atc.Space

	lockFactory lock.LockFactory
	conn        Conn
}

func (r *resourceConfig) ID() int                                   { return r.id }
func (r *resourceConfig) CreatedByResourceCache() UsedResourceCache { return r.createdByResourceCache }
func (r *resourceConfig) CreatedByBaseResourceType() *UsedBaseResourceType {
	return r.createdByBaseResourceType
}
func (r *resourceConfig) DefaultSpace() atc.Space { return r.defaultSpace }
func (r *resourceConfig) CheckError() error       { return r.checkError }

func (r *resourceConfig) OriginBaseResourceType() *UsedBaseResourceType {
	if r.createdByBaseResourceType != nil {
		return r.createdByBaseResourceType
	}
	return r.createdByResourceCache.ResourceConfig().OriginBaseResourceType()
}

func (r *resourceConfig) FindVersion(space atc.Space, version atc.Version) (ResourceVersion, bool, error) {
	rcv := &resourceVersion{
		resourceConfig: r,
		conn:           r.conn,
	}

	versionByte, err := json.Marshal(version)
	if err != nil {
		return nil, false, err
	}

	row := resourceVersionQuery.
		Where(sq.Eq{
			"s.resource_config_id": r.id,
			"s.name":               space,
		}).
		Where(sq.Expr("s.id = v.space_id")).
		Where(sq.Expr("v.version_md5 = md5(?)", versionByte)).
		RunWith(r.conn).
		QueryRow()

	err = scanResourceVersion(rcv, row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	return rcv, true, nil
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

	err = saveResourceVersion(tx, r, space, version, metadata, true)
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

// XXX: Does the default space need to exist???
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

	latestIdQuery := sq.Expr(`( SELECT v.id FROM resource_versions v, spaces s
																	WHERE version_md5 = md5(?)
																	AND s.id = v.space_id
																	AND s.name = ?
																	AND s.resource_config_id = ? )`, versionBlob, space, r.id)

	_, err = psql.Update("spaces").
		Set("latest_resource_version_id", latestIdQuery).
		Where(sq.Eq{
			"resource_config_id": r.id,
			"name":               space,
		}).
		RunWith(r.conn).
		Exec()

	return err
}

func (r *resourceConfig) FinishSavingVersions() error {
	_, err := r.conn.Exec(`UPDATE resource_versions
		SET partial = false
		FROM spaces
		WHERE spaces.resource_config_id = $1
		AND spaces.id = resource_versions.space_id;`, r.id)
	if err != nil {
		return err
	}

	err = bumpCacheIndexForPipelinesUsingResourceConfig(r.conn, r.id)
	if err != nil {
		return err
	}

	return nil
}

func (r *resourceConfig) LatestVersions() ([]ResourceVersion, error) {
	rows, err := resourceVersionQuery.
		Where(sq.Eq{
			"s.resource_config_id": r.id,
		}).
		Where(sq.Expr("s.latest_resource_version_id = v.id")).
		RunWith(r.conn).
		Query()
	if err != nil {
		return nil, err
	}

	versions := []ResourceVersion{}
	for rows.Next() {
		rcv := &resourceVersion{
			conn:           r.conn,
			resourceConfig: r,
		}

		err := scanResourceVersion(rcv, rows)
		if err != nil {
			return nil, err
		}

		versions = append(versions, rcv)
	}

	return versions, nil
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

func (r *resourceConfig) AcquireResourceCheckingLock(
	logger lager.Logger,
) (lock.Lock, bool, error) {
	return r.lockFactory.Acquire(
		logger,
		lock.NewResourceConfigCheckingLockID(r.id),
	)
}

func (r *resourceConfig) UpdateLastChecked(
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

func (r *resourceConfig) UpdateLastCheckFinished() (bool, error) {
	tx, err := r.conn.Begin()
	if err != nil {
		return false, err
	}

	defer Rollback(tx)

	updated, err := checkIfRowsUpdated(tx, `
			UPDATE resource_configs
			SET last_check_finished = now()
			WHERE id = $1
		`, r.id)
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

func saveResourceVersion(tx Tx, r ResourceConfig, space atc.Space, version atc.Version, metadata atc.Metadata, partial bool) error {
	versionJSON, err := json.Marshal(version)
	if err != nil {
		return err
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	if len(metadata) > 0 {
		_, err = tx.Exec(`
		INSERT INTO resource_versions (space_id, version, version_md5, metadata, partial)
		SELECT s.id, $2, md5($3), $4, $6
		FROM spaces s WHERE s.resource_config_id = $1 AND s.name = $5
		ON CONFLICT (space_id, version_md5) DO UPDATE SET metadata = $4
		`, r.ID(), string(versionJSON), string(versionJSON), string(metadataJSON), string(space), partial)
		if err != nil {
			return err
		}
	} else {
		_, err = tx.Exec(`
		INSERT INTO resource_versions (space_id, version, version_md5, metadata, partial)
		SELECT s.id, $2, md5($3), $4, $6
		FROM spaces s WHERE s.resource_config_id = $1 AND s.name = $5
		ON CONFLICT (space_id, version_md5) DO NOTHING
		`, r.ID(), string(versionJSON), string(versionJSON), string(metadataJSON), string(space), partial)
		if err != nil {
			return err
		}
	}

	return nil
}

// increment the check order if the version's check order is less than the
// current max. This will fix the case of a check from an old version causing
// the desired order to change; existing versions will be re-ordered since
// we add them in the desired order.
func incrementCheckOrder(tx Tx, r ResourceConfig, space atc.Space, version string) error {
	_, err := tx.Exec(`
		WITH max_checkorder AS (
			SELECT max(check_order) co
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

// XXX: Should this only bump if the resource is using the space associated???
func bumpCacheIndexForPipelinesUsingResourceConfig(conn Conn, rcID int) error {
	rows, err := psql.Select("p.id").
		From("pipelines p").
		Join("resources r ON r.pipeline_id = p.id").
		Where(sq.Eq{
			"r.resource_config_id": rcID,
		}).
		RunWith(conn).
		Query()
	if err != nil {
		return err
	}

	var pipelines []int
	for rows.Next() {
		var pid int
		err = rows.Scan(&pid)
		if err != nil {
			return err
		}

		pipelines = append(pipelines, pid)
	}

	for _, p := range pipelines {
		_, err := psql.Update("pipelines").
			Set("cache_index", sq.Expr("cache_index + 1")).
			Where(sq.Eq{
				"id": p,
			}).
			RunWith(conn).
			Exec()
		if err != nil {
			return err
		}
	}

	return nil
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

	id, checkErr, defaultSpace, found, err := r.findWithParentID(tx, parentColumnName, parentID)
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
	rc.checkError = checkErr
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

	id, checkErr, defaultSpace, found, err := r.findWithParentID(tx, parentColumnName, parentID)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	rc.id = id
	rc.checkError = checkErr
	rc.defaultSpace = defaultSpace

	return rc, true, nil
}

func (r *ResourceConfigDescriptor) findWithParentID(tx Tx, parentColumnName string, parentID int) (int, error, atc.Space, bool, error) {
	var id int
	var whereClause sq.Eq
	var checkErr error
	var defaultSpace atc.Space
	var checkErrBlob sql.NullString
	var defSpace sql.NullString

	err := psql.Select("id, check_error, default_space").
		From("resource_configs").
		Where(sq.Eq{
			parentColumnName: parentID,
			"source_hash":    mapHash(r.Source),
		}).
		Where(whereClause).
		Suffix("FOR SHARE").
		RunWith(tx).
		QueryRow().
		Scan(&id, &checkErrBlob, &defSpace)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil, "", false, nil
		}

		return 0, nil, "", false, err
	}

	if checkErrBlob.Valid {
		checkErr = errors.New(checkErrBlob.String)
	}

	if defSpace.Valid {
		defaultSpace = atc.Space(defSpace.String)
	}

	return id, checkErr, defaultSpace, true, nil
}
