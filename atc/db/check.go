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
	"github.com/lib/pq"
)

type CheckStatus string

const (
	CheckStatusStarted   CheckStatus = "started"
	CheckStatusSucceeded CheckStatus = "succeeded"
	CheckStatusErrored   CheckStatus = "errored"
)

//go:generate counterfeiter . Check

type Check interface {
	ID() int
	ResourceConfigScopeID() int
	ResourceConfigID() int
	BaseResourceTypeID() int
	Schema() string
	Plan() atc.Plan
	CreateTime() time.Time
	StartTime() time.Time
	EndTime() time.Time
	Status() CheckStatus
	CheckError() error

	Start() error
	Finish() error
	FinishWithError(err error) error

	SaveVersions([]atc.Version) error
	AllCheckables() ([]Checkable, error)
	AcquireTrackingLock(lager.Logger) (lock.Lock, bool, error)
	Reload() (bool, error)
}

var checksQuery = psql.Select("c.id, c.resource_config_scope_id, c.resource_config_id, c.base_resource_type_id, c.status, c.schema, c.create_time, c.start_time, c.end_time, c.plan, c.nonce, c.check_error").
	From("checks c")

type check struct {
	id                    int
	resourceConfigScopeID int
	resourceConfigID      int
	baseResourceTypeID    int

	status     CheckStatus
	schema     string
	plan       atc.Plan
	checkError error

	createTime time.Time
	startTime  time.Time
	endTime    time.Time

	conn        Conn
	lockFactory lock.LockFactory
}

func (c *check) ID() int                    { return c.id }
func (c *check) ResourceConfigScopeID() int { return c.resourceConfigScopeID }
func (c *check) ResourceConfigID() int      { return c.resourceConfigID }
func (c *check) BaseResourceTypeID() int    { return c.baseResourceTypeID }
func (c *check) Status() CheckStatus        { return c.status }
func (c *check) Schema() string             { return c.schema }
func (c *check) Plan() atc.Plan             { return c.plan }
func (c *check) CreateTime() time.Time      { return c.createTime }
func (c *check) StartTime() time.Time       { return c.startTime }
func (c *check) EndTime() time.Time         { return c.endTime }
func (c *check) CheckError() error          { return c.checkError }

func (c *check) Reload() (bool, error) {
	row := checksQuery.Where(sq.Eq{"c.id": c.id}).
		RunWith(c.conn).
		QueryRow()

	err := scanCheck(c, row)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (c *check) Start() error {
	tx, err := c.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	now := time.Now()
	_, err = psql.Update("checks").
		Set("start_time", now).
		Where(sq.Eq{
			"id": c.id,
		}).
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	_, err = psql.Update("resource_config_scopes").
		Set("last_check_start_time", now).
		Where(sq.Eq{
			"id": c.resourceConfigScopeID,
		}).
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (c *check) Finish() error {
	return c.finish(CheckStatusSucceeded, nil)
}

func (c *check) FinishWithError(err error) error {
	return c.finish(CheckStatusErrored, err)
}

func (c *check) finish(status CheckStatus, checkError error) error {
	tx, err := c.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	now := time.Now()
	builder := psql.Update("checks").
		Set("status", status).
		Set("end_time", now).
		Where(sq.Eq{
			"id": c.id,
		})

	if checkError != nil {
		builder = builder.Set("check_error", checkError.Error())
	}

	_, err = builder.
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	builder = psql.Update("resource_config_scopes").
		Set("last_check_end_time", now).
		Where(sq.Eq{
			"id": c.resourceConfigScopeID,
		})

	if checkError != nil {
		builder = builder.Set("check_error", checkError.Error())
	}

	_, err = builder.
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (c *check) AcquireTrackingLock(logger lager.Logger) (lock.Lock, bool, error) {
	return c.lockFactory.Acquire(
		logger,
		lock.NewResourceConfigCheckingLockID(c.ResourceConfigID()),
	)
}

func (c *check) AllCheckables() ([]Checkable, error) {
	var checkables []Checkable

	rows, err := resourcesQuery.
		Where(sq.Eq{
			"r.resource_config_scope_id": c.resourceConfigScopeID,
		}).
		RunWith(c.conn).
		Query()

	if err != nil {
		return nil, err
	}

	defer Close(rows)

	for rows.Next() {
		r := &resource{
			conn:        c.conn,
			lockFactory: c.lockFactory,
		}

		err = scanResource(r, rows)
		if err != nil {
			return nil, err
		}

		checkables = append(checkables, r)
	}

	rows, err = resourceTypesQuery.
		Where(sq.Eq{
			"ro.id": c.resourceConfigScopeID,
		}).
		RunWith(c.conn).
		Query()

	if err != nil {
		return nil, err
	}

	defer Close(rows)

	for rows.Next() {
		r := &resourceType{
			conn:        c.conn,
			lockFactory: c.lockFactory,
		}

		err = scanResourceType(r, rows)
		if err != nil {
			return nil, err
		}

		checkables = append(checkables, r)
	}

	return checkables, nil
}

func (c *check) SaveVersions(versions []atc.Version) error {
	return saveVersions(c.conn, c.resourceConfigScopeID, versions)
}

func scanCheck(c *check, row scannable) error {
	var (
		resourceConfigScopeID, resourceConfigID, baseResourceTypeID sql.NullInt64
		createTime, startTime, endTime                              pq.NullTime
		schema, plan, nonce, checkError                             sql.NullString
		status                                                      string
	)

	err := row.Scan(&c.id, &resourceConfigScopeID, &resourceConfigID, &baseResourceTypeID, &status, &schema, &createTime, &startTime, &endTime, &plan, &nonce, &checkError)
	if err != nil {
		return err
	}

	var noncense *string
	if nonce.Valid {
		noncense = &nonce.String
	}

	es := c.conn.EncryptionStrategy()
	decryptedConfig, err := es.Decrypt(string(plan.String), noncense)
	if err != nil {
		return err
	}

	err = json.Unmarshal(decryptedConfig, &c.plan)
	if err != nil {
		return err
	}

	if checkError.Valid {
		c.checkError = errors.New(checkError.String)
	} else {
		c.checkError = nil
	}

	c.status = CheckStatus(status)
	c.schema = schema.String
	c.resourceConfigScopeID = int(resourceConfigScopeID.Int64)
	c.resourceConfigID = int(resourceConfigID.Int64)
	c.baseResourceTypeID = int(baseResourceTypeID.Int64)
	c.createTime = createTime.Time
	c.startTime = startTime.Time
	c.endTime = endTime.Time

	return nil
}
