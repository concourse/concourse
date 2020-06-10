package db

import (
	"database/sql"
	"encoding/json"
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

	SaveVersions(SpanContext, []atc.Version) error
	FindVersion(atc.Version) (ResourceConfigVersion, bool, error)
	LatestVersion() (ResourceConfigVersion, bool, error)

	SetCheckError(error) error

	AcquireResourceCheckingLock(
		logger lager.Logger,
	) (lock.Lock, bool, error)

	UpdateLastCheckStartTime(
		interval time.Duration,
		immediate bool,
	) (bool, error)

	UpdateLastCheckEndTime() (bool, error)
}

type resourceConfigScope struct {
	id             int
	resource       Resource
	resourceConfig ResourceConfig
	checkError     error

	conn        Conn
	lockFactory lock.LockFactory
}

func (r *resourceConfigScope) ID() int                        { return r.id }
func (r *resourceConfigScope) Resource() Resource             { return r.resource }
func (r *resourceConfigScope) ResourceConfig() ResourceConfig { return r.resourceConfig }
func (r *resourceConfigScope) CheckError() error              { return r.checkError }

// SaveVersions stores a list of version in the db for a resource config
// Each version will also have its check order field updated and the
// Cache index for pipelines using the resource config will be bumped.
//
// In the case of a check resource from an older version, the versions
// that already exist in the DB will be re-ordered using
// incrementCheckOrder to input the correct check order
func (r *resourceConfigScope) SaveVersions(spanContext SpanContext, versions []atc.Version) error {
	return saveVersions(r.conn, r.ID(), versions, spanContext)
}

func saveVersions(conn Conn, rcsID int, versions []atc.Version, spanContext SpanContext) error {
	tx, err := conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	var containsNewVersion bool
	for _, version := range versions {
		newVersion, err := saveResourceVersion(tx, rcsID, version, nil, spanContext)
		if err != nil {
			return err
		}

		containsNewVersion = containsNewVersion || newVersion
	}

	if containsNewVersion {
		// bump the check order of all the versions returned by the check if there
		// is at least one new version within the set of returned versions
		for _, version := range versions {
			versionJSON, err := json.Marshal(version)
			if err != nil {
				return err
			}

			err = incrementCheckOrder(tx, rcsID, string(versionJSON))
			if err != nil {
				return err
			}
		}

		err = requestScheduleForJobsUsingResourceConfigScope(tx, rcsID)
		if err != nil {
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (r *resourceConfigScope) FindVersion(v atc.Version) (ResourceConfigVersion, bool, error) {
	rcv := &resourceConfigVersion{
		resourceConfigScope: r,
		conn:                r.conn,
	}

	versionByte, err := json.Marshal(v)
	if err != nil {
		return nil, false, err
	}

	row := resourceConfigVersionQuery.
		Where(sq.Eq{
			"v.resource_config_scope_id": r.id,
		}).
		Where(sq.Expr("v.version_md5 = md5(?)", versionByte)).
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

func (r *resourceConfigScope) LatestVersion() (ResourceConfigVersion, bool, error) {
	rcv := &resourceConfigVersion{
		conn:                r.conn,
		resourceConfigScope: r,
	}

	row := resourceConfigVersionQuery.
		Where(sq.Eq{"v.resource_config_scope_id": r.id}).
		OrderBy("v.check_order DESC").
		Limit(1).
		RunWith(r.conn).
		QueryRow()

	err := scanResourceConfigVersion(rcv, row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	return rcv, true, nil
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
) (lock.Lock, bool, error) {
	return r.lockFactory.Acquire(
		logger,
		lock.NewResourceConfigCheckingLockID(r.resourceConfig.ID()),
	)
}

func (r *resourceConfigScope) UpdateLastCheckStartTime(
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
		condition = "AND now() - last_check_start_time > ($2 || ' SECONDS')::INTERVAL"
		params = append(params, interval.Seconds())
	}

	updated, err := checkIfRowsUpdated(tx, `
			UPDATE resource_config_scopes
			SET last_check_start_time = now()
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

func (r *resourceConfigScope) UpdateLastCheckEndTime() (bool, error) {
	tx, err := r.conn.Begin()
	if err != nil {
		return false, err
	}

	defer Rollback(tx)

	updated, err := checkIfRowsUpdated(tx, `
			UPDATE resource_config_scopes
			SET last_check_end_time = now()
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

func saveResourceVersion(tx Tx, rcsID int, version atc.Version, metadata ResourceConfigMetadataFields, spanContext SpanContext) (bool, error) {
	versionJSON, err := json.Marshal(version)
	if err != nil {
		return false, err
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return false, err
	}

	spanContextJSON, err := json.Marshal(spanContext)
	if err != nil {
		return false, err
	}

	var checkOrder int
	err = tx.QueryRow(`
		INSERT INTO resource_config_versions (resource_config_scope_id, version, version_md5, metadata, span_context)
		SELECT $1, $2, md5($3), $4, $5
		ON CONFLICT (resource_config_scope_id, version_md5)
		DO UPDATE SET metadata = COALESCE(NULLIF(excluded.metadata, 'null'::jsonb), resource_config_versions.metadata)
		RETURNING check_order
		`, rcsID, string(versionJSON), string(versionJSON), string(metadataJSON), string(spanContextJSON)).Scan(&checkOrder)
	if err != nil {
		return false, err
	}

	return checkOrder == 0, nil
}

// increment the check order if the version's check order is less than the
// current max. This will fix the case of a check from an old version causing
// the desired order to change; existing versions will be re-ordered since
// we add them in the desired order.
func incrementCheckOrder(tx Tx, rcsID int, version string) error {
	_, err := tx.Exec(`
		WITH max_checkorder AS (
			SELECT max(check_order) co
			FROM resource_config_versions
			WHERE resource_config_scope_id = $1
		)

		UPDATE resource_config_versions
		SET check_order = mc.co + 1
		FROM max_checkorder mc
		WHERE resource_config_scope_id = $1
		AND version_md5 = md5($2)
		AND check_order <= mc.co;`, rcsID, version)
	return err
}

// The SELECT query orders the jobs for updating to prevent deadlocking.
// Updating multiple rows using a SELECT subquery does not preserve the same
// order for the updates, which can lead to deadlocking.
func requestScheduleForJobsUsingResourceConfigScope(tx Tx, rcsID int) error {
	rows, err := psql.Select("DISTINCT j.job_id").
		From("job_inputs j").
		Join("resources r ON r.id = j.resource_id").
		Where(sq.Eq{
			"r.resource_config_scope_id": rcsID,
			"j.passed_job_id":            nil,
		}).
		OrderBy("j.job_id DESC").
		RunWith(tx).
		Query()
	if err != nil {
		return err
	}

	var jobIDs []int
	for rows.Next() {
		var id int
		err = rows.Scan(&id)
		if err != nil {
			return err
		}

		jobIDs = append(jobIDs, id)
	}

	for _, jID := range jobIDs {
		_, err := psql.Update("jobs").
			Set("schedule_requested", sq.Expr("now()")).
			Where(sq.Eq{
				"id": jID,
			}).
			RunWith(tx).
			Exec()
		if err != nil {
			return err
		}
	}

	return nil
}
