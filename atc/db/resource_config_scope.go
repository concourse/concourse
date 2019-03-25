package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"code.cloudfoundry.org/lager"
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/lock"
)

//go:generate counterfeiter . ResourceConfigScope

// ResourceConfigScope represents the relationship between a possible pipeline resource and a resource config.
// When a resource is specified to have a unique version history either through its base resource type or its custom
// resource type, it results in its generated resource config to be scoped to the resource. This relationship is
// translated into its row in the resource config scopes table to have both the resource id and resource config id
// populated. When a resource has a shared version history, its resource config is not scoped to the (or any) resource
// and its row in the resource config scopes table will have the resource config id populated but a NULL value for
// the resource id. Resource versions will therefore be directly dependent on a resource config scope.
type ResourceConfigScope interface {
	ID() int
	Resource() Resource
	ResourceConfig() ResourceConfig
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
		interval time.Duration,
	) (lock.Lock, bool, error)

	UpdateLastChecked(
		interval time.Duration,
		immediate bool,
	) (bool, error)

	UpdateLastCheckFinished() (bool, error)
}

type resourceConfigScope struct {
	id             int
	resource       Resource
	resourceConfig ResourceConfig
	defaultSpace   atc.Space
	checkError     error

	conn        Conn
	lockFactory lock.LockFactory
}

func (r *resourceConfigScope) ID() int                        { return r.id }
func (r *resourceConfigScope) Resource() Resource             { return r.resource }
func (r *resourceConfigScope) ResourceConfig() ResourceConfig { return r.resourceConfig }
func (r *resourceConfigScope) DefaultSpace() atc.Space        { return r.defaultSpace }
func (r *resourceConfigScope) CheckError() error              { return r.checkError }

func (r *resourceConfigScope) FindVersion(space atc.Space, version atc.Version) (ResourceVersion, bool, error) {
	rcv := &resourceVersion{
		resourceConfigScope: r,
		conn:                r.conn,
	}

	versionByte, err := json.Marshal(version)
	if err != nil {
		return nil, false, err
	}

	row := resourceVersionQuery.
		Where(sq.Eq{
			"s.resource_config_scope_id": r.id,
			"s.name":                     space,
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
func (r *resourceConfigScope) SavePartialVersion(space atc.Space, version atc.Version, metadata atc.Metadata) error {
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
func (r *resourceConfigScope) SaveDefaultSpace(defaultSpace atc.Space) error {
	_, err := psql.Update("resource_config_scopes").
		Set("default_space", defaultSpace).
		Where(sq.Eq{"id": r.id}).
		RunWith(r.conn).
		Exec()
	return err
}

func (r *resourceConfigScope) SaveSpace(space atc.Space) error {
	_, err := psql.Insert("spaces").
		Columns("resource_config_scope_id", "name").
		Values(r.id, space).
		Suffix("ON CONFLICT DO NOTHING").
		RunWith(r.conn).
		Exec()
	return err
}

func (r *resourceConfigScope) SaveSpaceLatestVersion(space atc.Space, version atc.Version) error {
	versionBlob, err := json.Marshal(version)
	if err != nil {
		return err
	}

	latestIdQuery := fmt.Sprintf(`( SELECT v.id FROM resource_versions v, spaces s
																	WHERE version_md5 = md5('%s')
																	AND s.id = v.space_id
																	AND s.name = '%s'
																	AND s.resource_config_scope_id = %d )`, versionBlob, space, r.id)

	_, err = psql.Update("spaces").
		Set("latest_resource_version_id", sq.Expr(latestIdQuery)).
		Where(sq.Eq{
			"resource_config_scope_id": r.id,
			"name":                     space,
		}).
		RunWith(r.conn).
		Exec()

	return err
}

func (r *resourceConfigScope) FinishSavingVersions() error {
	_, err := r.conn.Exec(`UPDATE resource_versions
		SET partial = false
		FROM spaces
		WHERE spaces.resource_config_scope_id = $1
		AND spaces.id = resource_versions.space_id;`, r.id)
	if err != nil {
		return err
	}

	err = bumpCacheIndexForPipelinesUsingResourceConfigScope(r.conn, r.id)
	if err != nil {
		return err
	}

	return nil
}

func (r *resourceConfigScope) LatestVersions() ([]ResourceVersion, error) {
	rows, err := resourceVersionQuery.
		Where(sq.Eq{
			"s.resource_config_scope_id": r.id,
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

	versions := []ResourceVersion{}
	for rows.Next() {
		rcv := &resourceVersion{
			conn:                r.conn,
			resourceConfigScope: r,
		}

		err := scanResourceVersion(rcv, rows)
		if err != nil {
			return nil, err
		}

		versions = append(versions, rcv)
	}

	return versions, nil
}

func (r *resourceConfigScope) SetCheckError(cause error) error {
	var err error

	if cause == nil {
		_, err = psql.Update("resource_config_scopes").
			Set("check_error", nil).
			Where(sq.Eq{"id": r.id}).
			RunWith(r.conn).
			Exec()
	} else {
		_, err = psql.Update("resource_config_scopes").
			Set("check_error", cause.Error()).
			Where(sq.Eq{"id": r.id}).
			RunWith(r.conn).
			Exec()
	}

	return err
}

func (r *resourceConfigScope) AcquireResourceCheckingLock(
	logger lager.Logger,
	interval time.Duration,
) (lock.Lock, bool, error) {
	return r.lockFactory.Acquire(
		logger,
		lock.NewResourceConfigCheckingLockID(r.resourceConfig.ID()),
	)
}

func (r *resourceConfigScope) UpdateLastChecked(
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
			UPDATE resource_config_scopes
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

func (r *resourceConfigScope) UpdateLastCheckFinished() (bool, error) {
	tx, err := r.conn.Begin()
	if err != nil {
		return false, err
	}

	defer Rollback(tx)

	updated, err := checkIfRowsUpdated(tx, `
			UPDATE resource_config_scopes
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

func saveResourceVersion(tx Tx, r ResourceConfigScope, space atc.Space, version atc.Version, metadata atc.Metadata, partial bool) error {
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
		FROM spaces s WHERE s.resource_config_scope_id = $1 AND s.name = $5
		ON CONFLICT (space_id, version_md5) DO UPDATE SET metadata = $4
		`, r.ID(), string(versionJSON), string(versionJSON), string(metadataJSON), string(space), partial)
		if err != nil {
			return err
		}
	} else {
		_, err = tx.Exec(`
		INSERT INTO resource_versions (space_id, version, version_md5, metadata, partial)
		SELECT s.id, $2, md5($3), $4, $6
		FROM spaces s WHERE s.resource_config_scope_id = $1 AND s.name = $5
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
func incrementCheckOrder(tx Tx, r ResourceConfigScope, space atc.Space, version string) error {
	_, err := tx.Exec(`
		WITH max_checkorder AS (
			SELECT max(check_order) co
			FROM resource_versions v, spaces s
			WHERE s.resource_config_scope_id = $1
			AND s.name = $3
			AND s.id = v.space_id
		)

		UPDATE resource_versions
		SET check_order = mc.co + 1
		FROM max_checkorder mc, spaces s
		WHERE space_id = s.id
		AND s.resource_config_scope_id = $1
		AND s.name = $3
		AND version = $2
		AND check_order <= mc.co;`, r.ID(), version, space)
	return err
}

// XXX: Should this only bump if the resource is using the space associated???
func bumpCacheIndexForPipelinesUsingResourceConfigScope(conn Conn, rcsID int) error {
	rows, err := psql.Select("p.id").
		From("pipelines p").
		Join("resources r ON r.pipeline_id = p.id").
		Where(sq.Eq{
			"r.resource_config_scope_id": rcsID,
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
