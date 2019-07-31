package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"code.cloudfoundry.org/lager"
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
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
	CurrentPinnedVersion() atc.Version

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
	TryCreateCheck(Checkable, ResourceTypes, atc.Version, bool) (Check, bool, error)
	Resources() ([]Resource, error)
	ResourceTypes() ([]ResourceType, error)
	AcquireScanningLock(lager.Logger) (lock.Lock, bool, error)
	NotifyChecker() error
}

type checkFactory struct {
	conn        Conn
	lockFactory lock.LockFactory

	secrets             creds.Secrets
	defaultCheckTimeout time.Duration
}

func NewCheckFactory(
	conn Conn,
	lockFactory lock.LockFactory,
	secrets creds.Secrets,
	defaultCheckTimeout time.Duration,
) CheckFactory {
	return &checkFactory{
		conn:        conn,
		lockFactory: lockFactory,

		secrets:             secrets,
		defaultCheckTimeout: defaultCheckTimeout,
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

func (c *checkFactory) TryCreateCheck(checkable Checkable, resourceTypes ResourceTypes, fromVersion atc.Version, manuallyTriggered bool) (Check, bool, error) {

	var err error

	parentType, found := resourceTypes.Parent(checkable)
	if found {
		if parentType.Version() == nil {
			return nil, false, errors.New("parent type has no version")
		}
	}

	timeout := c.defaultCheckTimeout
	if to := checkable.CheckTimeout(); to != "" {
		timeout, err = time.ParseDuration(to)
		if err != nil {
			return nil, false, err
		}
	}

	variables := creds.NewVariables(
		c.secrets,
		checkable.TeamName(),
		checkable.PipelineName(),
	)

	source, err := creds.NewSource(variables, checkable.Source()).Evaluate()
	if err != nil {
		return nil, false, err
	}

	filteredTypes := resourceTypes.Filter(checkable).Deserialize()
	versionedResourceTypes, err := creds.NewVersionedResourceTypes(variables, filteredTypes).Evaluate()
	if err != nil {
		return nil, false, err
	}

	// This could have changed based on new variable interpolation so update it
	resourceConfigScope, err := checkable.SetResourceConfig(source, versionedResourceTypes)
	if err != nil {
		return nil, false, err
	}

	if fromVersion == nil {
		rcv, found, err := resourceConfigScope.LatestVersion()
		if err != nil {
			return nil, false, err
		}

		if found {
			fromVersion = atc.Version(rcv.Version())
		}
	}

	plan := atc.Plan{
		Check: &atc.CheckPlan{
			Name:        checkable.Name(),
			Type:        checkable.Type(),
			Source:      source,
			Tags:        checkable.Tags(),
			Timeout:     timeout.String(),
			FromVersion: fromVersion,

			VersionedResourceTypes: versionedResourceTypes,
		},
	}

	check, created, err := c.CreateCheck(
		resourceConfigScope.ID(),
		resourceConfigScope.ResourceConfig().ID(),
		resourceConfigScope.ResourceConfig().OriginBaseResourceType().ID,
		checkable.TeamID(),
		manuallyTriggered,
		plan,
	)
	if err != nil {
		return nil, false, err
	}

	return check, created, nil
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
