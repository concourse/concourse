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
	"go.opentelemetry.io/otel/api/propagators"
)

type CheckStatus string

const (
	CheckStatusStarted   CheckStatus = "started"
	CheckStatusSucceeded CheckStatus = "succeeded"
	CheckStatusErrored   CheckStatus = "errored"
)

//go:generate counterfeiter . Check

type Check interface {
	PipelineRef

	ID() int
	TeamID() int
	TeamName() string
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
	ManuallyTriggered() bool

	Start() error
	Finish() error
	FinishWithError(err error) error

	SaveVersions(SpanContext, []atc.Version) error
	AllCheckables() ([]Checkable, error)
	AcquireTrackingLock(lager.Logger) (lock.Lock, bool, error)
	Reload() (bool, error)

	SpanContext() propagators.Supplier
}

var checksQuery = psql.Select(
	"c.id",
	"c.resource_config_scope_id",
	"c.status",
	"c.schema",
	"c.create_time",
	"c.start_time",
	"c.end_time",
	"c.plan",
	"c.nonce",
	"c.check_error",
	"c.metadata",
	"c.span_context",
	"c.manually_triggered",
).
	From("checks c")

type check struct {
	pipelineRef

	id                    int
	resourceConfigScopeID int
	metadata              CheckMetadata
	manuallyTriggered     bool

	status     CheckStatus
	schema     string
	plan       atc.Plan
	checkError error

	createTime time.Time
	startTime  time.Time
	endTime    time.Time

	spanContext SpanContext
}

type CheckMetadata struct {
	TeamID               int              `json:"team_id"`
	TeamName             string           `json:"team_name"`
	PipelineID           int              `json:"pipeline_id"`
	PipelineName         string           `json:"pipeline_name"`
	PipelineInstanceVars atc.InstanceVars `json:"pipeline_instance_vars,omitempty"`
	ResourceConfigID     int              `json:"resource_config_id"`
	BaseResourceTypeID   int              `json:"base_resource_type_id"`
}

func newEmptyCheck(conn Conn, lockFactory lock.LockFactory) *check {
	return &check{pipelineRef: pipelineRef{conn: conn, lockFactory: lockFactory}}
}

func (c *check) ID() int                    { return c.id }
func (c *check) ResourceConfigScopeID() int { return c.resourceConfigScopeID }
func (c *check) Status() CheckStatus        { return c.status }
func (c *check) Schema() string             { return c.schema }
func (c *check) Plan() atc.Plan             { return c.plan }
func (c *check) CreateTime() time.Time      { return c.createTime }
func (c *check) StartTime() time.Time       { return c.startTime }
func (c *check) EndTime() time.Time         { return c.endTime }
func (c *check) CheckError() error          { return c.checkError }
func (c *check) ManuallyTriggered() bool    { return c.manuallyTriggered }

func (c *check) TeamID() int {
	return c.metadata.TeamID
}

func (c *check) TeamName() string {
	return c.metadata.TeamName
}

func (c *check) ResourceConfigID() int {
	return c.metadata.ResourceConfigID
}

func (c *check) BaseResourceTypeID() int {
	return c.metadata.BaseResourceTypeID
}

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

	var startTime time.Time
	err = psql.Update("checks").
		Set("start_time", sq.Expr("now()")).
		Where(sq.Eq{
			"id": c.id,
		}).
		Suffix("RETURNING start_time").
		RunWith(tx).
		QueryRow().
		Scan(&startTime)
	if err != nil {
		return err
	}

	_, err = psql.Update("resource_config_scopes").
		Set("last_check_start_time", sq.Expr("now()")).
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

	c.startTime = startTime

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

	var endTime time.Time
	builder := psql.Update("checks").
		Set("status", status).
		Set("end_time", sq.Expr("now()")).
		Where(sq.Eq{
			"id": c.id,
		})

	if checkError != nil {
		builder = builder.Set("check_error", checkError.Error())
	}

	err = builder.
		Suffix("RETURNING end_time").
		RunWith(tx).
		QueryRow().
		Scan(&endTime)
	if err != nil {
		return err
	}

	builder = psql.Update("resource_config_scopes").
		Set("last_check_end_time", sq.Expr("now()")).
		Where(sq.Eq{
			"id": c.resourceConfigScopeID,
		})

	if checkError != nil {
		builder = builder.Set("check_error", checkError.Error())
	} else {
		builder = builder.Set("check_error", nil)
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

	c.endTime = endTime
	c.status = status

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

	tx, err := c.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)
	rows, err := resourcesQuery.
		Where(sq.Eq{
			"r.resource_config_scope_id": c.resourceConfigScopeID,
		}).
		RunWith(tx).
		Query()

	if err != nil {
		return nil, err
	}

	defer Close(rows)

	for rows.Next() {
		r := newEmptyResource(c.conn, c.lockFactory)
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
		RunWith(tx).
		Query()

	if err != nil {
		return nil, err
	}

	defer Close(rows)

	for rows.Next() {
		r := newEmptyResourceType(c.conn, c.lockFactory)
		err = scanResourceType(r, rows)
		if err != nil {
			return nil, err
		}

		checkables = append(checkables, r)
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return checkables, nil
}

func (c *check) SaveVersions(spanContext SpanContext, versions []atc.Version) error {
	return saveVersions(c.conn, c.resourceConfigScopeID, versions, spanContext)
}

func (c *check) SpanContext() propagators.Supplier {
	return c.spanContext
}

func scanCheck(c *check, row scannable) error {
	var (
		createTime, startTime, endTime  pq.NullTime
		schema, plan, nonce, checkError sql.NullString
		status                          string
		metadata, spanContext           sql.NullString
	)

	err := row.Scan(
		&c.id,
		&c.resourceConfigScopeID,
		&status,
		&schema,
		&createTime,
		&startTime,
		&endTime,
		&plan,
		&nonce,
		&checkError,
		&metadata,
		&spanContext,
		&c.manuallyTriggered,
	)
	if err != nil {
		return err
	}

	var noncense *string
	if nonce.Valid {
		noncense = &nonce.String
	}

	es := c.conn.EncryptionStrategy()
	decryptedPlan, err := es.Decrypt(string(plan.String), noncense)
	if err != nil {
		return err
	}

	if len(decryptedPlan) > 0 {
		err = json.Unmarshal(decryptedPlan, &c.plan)
		if err != nil {
			return err
		}
	}

	if len(metadata.String) > 0 {
		err = json.Unmarshal([]byte(metadata.String), &c.metadata)
		if err != nil {
			return err
		}
	}

	if spanContext.Valid {
		err = json.Unmarshal([]byte(spanContext.String), &c.spanContext)
		if err != nil {
			return err
		}
	}

	c.pipelineID = c.metadata.PipelineID
	c.pipelineName = c.metadata.PipelineName
	c.pipelineInstanceVars = c.metadata.PipelineInstanceVars

	if checkError.Valid {
		c.checkError = errors.New(checkError.String)
	} else {
		c.checkError = nil
	}

	c.status = CheckStatus(status)
	c.schema = schema.String
	c.createTime = createTime.Time
	c.startTime = startTime.Time
	c.endTime = endTime.Time

	return nil
}
