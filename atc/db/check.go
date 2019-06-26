package db

import (
	"database/sql"
	"encoding/json"
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

//go:generate counterfeiter . Checkable

type Checkable interface {
	Name() string
	TeamName() string
	PipelineName() string
	Type() string
	PipelineID() int
	Source() atc.Source
	Tags() atc.Tags
	CheckEvery() string
	CheckTimeout() string
	LastCheckEndTime() time.Time

	SetResourceConfig(
		atc.Source,
		atc.VersionedResourceTypes,
	) (ResourceConfigScope, error)

	SetCheckSetupError(error) error
}

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
	IsRunning() bool

	Start() error
	Finish() error
	FinishWithError(err error) error

	SaveVersions([]atc.Version) error
	AcquireTrackingLock(lager.Logger) (lock.Lock, bool, error)
}

const (
	CheckTypeResource     = "resource"
	CheckTypeResourceType = "resource_type"
)

var checksQuery = psql.Select("c.id, c.resource_config_scope_id, c.resource_config_id, c.base_resource_type_id, c.status, c.schema, c.create_time, c.start_time, c.end_time, c.plan, c.nonce").
	From("checks c")

type check struct {
	id                    int
	resourceConfigScopeID int
	resourceConfigID      int
	baseResourceTypeID    int

	status CheckStatus
	schema string
	plan   atc.Plan

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

// TODO update resource config scope last check start time
func (c *check) Start() error {
	tx, err := c.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	now := time.Now()
	_, err = psql.Update("checks").
		Set("status", CheckStatusStarted).
		Set("start_time", now).
		Where(sq.Eq{
			"id": c.id,
		}).
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	// TODO update resource config scope
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
	_, err = psql.Update("checks").
		Set("status", status).
		Set("end_time", now).
		Set("check_error", checkError).
		Where(sq.Eq{
			"id": c.id,
		}).
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	_, err = psql.Update("resource_config_scopes").
		Set("last_check_end_time", now).
		Set("check_error", checkError).
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

func (c *check) IsRunning() bool {
	return false
}

func (c *check) AcquireTrackingLock(logger lager.Logger) (lock.Lock, bool, error) {
	return c.lockFactory.Acquire(
		logger,
		lock.NewResourceConfigCheckingLockID(c.ResourceConfigID()),
	)
}

func (c *check) SaveVersions(versions []atc.Version) error {
	return saveVersions(c.conn, c.resourceConfigScopeID, versions)
}

func scanCheck(c *check, row scannable) error {
	var (
		resourceConfigScopeID, resourceConfigID, baseResourceTypeID sql.NullInt64
		createTime, startTime, endTime                              pq.NullTime
		schema, plan, nonce                                         sql.NullString
		status                                                      string
	)

	err := row.Scan(&c.id, &resourceConfigScopeID, &resourceConfigID, &baseResourceTypeID, &status, &schema, &createTime, &startTime, &endTime, &plan, &nonce)
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
