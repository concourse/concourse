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

//go:generate counterfeiter . Checkable

type Checkable interface {
	Name() string
	TeamID() int
	TeamName() string
	PipelineID() int
	PipelineName() string
	Type() string
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

//go:generate counterfeiter . CheckFactory

type CheckFactory interface {
	Check(int) (Check, bool, error)
	StartedChecks() ([]Check, error)
	CreateCheck(int, int, int, int, bool, atc.Plan) (Check, bool, error)
	Resources() ([]Resource, error)
	ResourceTypes() ([]ResourceType, error)
	AcquireScanningLock(lager.Logger) (lock.Lock, bool, error)
	NotifyChecker() error
}

type checkFactory struct {
	conn        Conn
	lockFactory lock.LockFactory
}

func NewCheckFactory(
	conn Conn,
	lockFactory lock.LockFactory,
) CheckFactory {
	return &checkFactory{
		conn:        conn,
		lockFactory: lockFactory,
	}
}

func (c *checkFactory) NotifyChecker() error {
	return c.conn.Bus().Notify("checker")
}

func (c *checkFactory) AcquireScanningLock(
	logger lager.Logger,
) (lock.Lock, bool, error) {
	return c.lockFactory.Acquire(
		logger,
		lock.NewResourceScanningLockID(),
	)
}

func (c *checkFactory) Check(id int) (Check, bool, error) {
	check := &check{
		conn:        c.conn,
		lockFactory: c.lockFactory,
	}

	row := checksQuery.
		Where(sq.Eq{"c.id": id}).
		RunWith(c.conn).
		QueryRow()

	err := scanCheck(check, row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	return check, true, nil
}
func (c *checkFactory) StartedChecks() ([]Check, error) {
	rows, err := checksQuery.
		Where(sq.Eq{"status": CheckStatusStarted}).
		OrderBy("c.id").
		RunWith(c.conn).
		Query()
	if err != nil {
		return nil, err
	}

	var checks []Check

	for rows.Next() {
		check := &check{conn: c.conn, lockFactory: c.lockFactory}

		err := scanCheck(check, rows)
		if err != nil {
			return nil, err
		}

		checks = append(checks, check)
	}

	return checks, nil
}

func (c *checkFactory) CreateCheck(resourceConfigScopeID, resourceConfigID, baseResourceTypeID, teamID int, manuallyTriggered bool, plan atc.Plan) (Check, bool, error) {
	tx, err := c.conn.Begin()
	if err != nil {
		return nil, false, err
	}

	defer Rollback(tx)

	planPayload, err := json.Marshal(plan)
	if err != nil {
		return nil, false, err
	}

	es := c.conn.EncryptionStrategy()
	encryptedPayload, nonce, err := es.Encrypt(planPayload)
	if err != nil {
		return nil, false, err
	}

	metadata, err := json.Marshal(map[string]interface{}{
		"team_id":               teamID,
		"resource_config_id":    resourceConfigID,
		"base_resource_type_id": baseResourceTypeID,
	})
	if err != nil {
		return nil, false, err
	}

	var id int
	var createTime time.Time
	err = psql.Insert("checks").
		Columns(
			"resource_config_scope_id",
			"schema",
			"status",
			"manually_triggered",
			"plan",
			"nonce",
			"metadata",
		).
		Values(
			resourceConfigScopeID,
			schema,
			CheckStatusStarted,
			manuallyTriggered,
			encryptedPayload,
			nonce,
			metadata,
		).
		Suffix(`
			ON CONFLICT DO NOTHING
			RETURNING id, create_time
		`).
		RunWith(tx).
		QueryRow().
		Scan(&id, &createTime)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, false, err
	}

	return &check{
		id:                    id,
		teamID:                teamID,
		resourceConfigScopeID: resourceConfigScopeID,
		resourceConfigID:      resourceConfigID,
		baseResourceTypeID:    baseResourceTypeID,
		schema:                schema,
		status:                CheckStatusStarted,
		plan:                  plan,
		createTime:            createTime,

		conn:        c.conn,
		lockFactory: c.lockFactory,
	}, true, err
}

func (c *checkFactory) Resources() ([]Resource, error) {
	var resources []Resource

	rows, err := resourcesQuery.
		Where(sq.Eq{"p.paused": false}).
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

		resources = append(resources, r)
	}

	return resources, nil
}

func (c *checkFactory) ResourceTypes() ([]ResourceType, error) {
	var resourceTypes []ResourceType

	rows, err := resourceTypesQuery.
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

		resourceTypes = append(resourceTypes, r)
	}

	return resourceTypes, nil
}
