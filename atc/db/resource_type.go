package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/lib/pq"
)

type ResourceTypeNotFoundError struct {
	ID int
}

func (e ResourceTypeNotFoundError) Error() string {
	return fmt.Sprintf("resource type not found: %d", e.ID)
}

//go:generate counterfeiter . ResourceType

type ResourceType interface {
	PipelineRef

	ID() int
	TeamID() int
	TeamName() string
	Name() string
	Type() string
	Privileged() bool
	Source() atc.Source
	Params() atc.Params
	Tags() atc.Tags
	CheckEvery() string
	CheckTimeout() string
	LastCheckStartTime() time.Time
	LastCheckEndTime() time.Time
	CheckSetupError() error
	CheckError() error
	UniqueVersionHistory() bool
	CurrentPinnedVersion() atc.Version
	ResourceConfigScopeID() int

	HasWebhook() bool

	SetResourceConfig(atc.Source, atc.VersionedResourceTypes) (ResourceConfigScope, error)
	SetCheckSetupError(error) error

	Version() atc.Version

	Reload() (bool, error)
}

type ResourceTypes []ResourceType

func (resourceTypes ResourceTypes) Parent(checkable Checkable) (ResourceType, bool) {
	for _, t := range resourceTypes {
		if t.PipelineID() == checkable.PipelineID() {
			if t != checkable && t.Name() == checkable.Type() {
				return t, true
			}
		}
	}
	return nil, false
}

func (resourceTypes ResourceTypes) Filter(checkable Checkable) ResourceTypes {
	var result ResourceTypes

	for {
		resourceType, found := resourceTypes.Parent(checkable)
		if !found {
			return result
		}

		result = append(result, resourceType)
		checkable = resourceType
	}
}

func (resourceTypes ResourceTypes) Deserialize() atc.VersionedResourceTypes {
	var versionedResourceTypes atc.VersionedResourceTypes

	for _, t := range resourceTypes {
		versionedResourceTypes = append(versionedResourceTypes, atc.VersionedResourceType{
			ResourceType: atc.ResourceType{
				Name:                 t.Name(),
				Type:                 t.Type(),
				Source:               t.Source(),
				Privileged:           t.Privileged(),
				CheckEvery:           t.CheckEvery(),
				Tags:                 t.Tags(),
				Params:               t.Params(),
				UniqueVersionHistory: t.UniqueVersionHistory(),
			},
			Version: t.Version(),
		})
	}

	return versionedResourceTypes
}

func (resourceTypes ResourceTypes) Configs() atc.ResourceTypes {
	var configs atc.ResourceTypes

	for _, r := range resourceTypes {
		configs = append(configs, atc.ResourceType{
			Name:                 r.Name(),
			Type:                 r.Type(),
			Source:               r.Source(),
			Privileged:           r.Privileged(),
			CheckEvery:           r.CheckEvery(),
			Tags:                 r.Tags(),
			Params:               r.Params(),
			UniqueVersionHistory: r.UniqueVersionHistory(),
		})
	}

	return configs
}

var resourceTypesQuery = psql.Select(
	"r.id",
	"r.pipeline_id",
	"r.name",
	"r.type",
	"r.config",
	"rcv.version",
	"r.nonce",
	"r.check_error",
	"p.name",
	"p.instance_vars",
	"t.id",
	"t.name",
	"ro.id",
	"ro.check_error",
	"ro.last_check_start_time",
	"ro.last_check_end_time",
).
	From("resource_types r").
	Join("pipelines p ON p.id = r.pipeline_id").
	Join("teams t ON t.id = p.team_id").
	LeftJoin("resource_configs c ON c.id = r.resource_config_id").
	LeftJoin("resource_config_scopes ro ON ro.resource_config_id = c.id").
	LeftJoin(`LATERAL (
		SELECT rcv.*
		FROM resource_config_versions rcv
		WHERE rcv.resource_config_scope_id = ro.id AND rcv.check_order != 0
		ORDER BY rcv.check_order DESC
		LIMIT 1
	) AS rcv ON true`).
	Where(sq.Eq{"r.active": true})

type resourceType struct {
	pipelineRef

	id                    int
	teamID                int
	resourceConfigScopeID int
	teamName              string
	name                  string
	type_                 string
	privileged            bool
	source                atc.Source
	params                atc.Params
	tags                  atc.Tags
	version               atc.Version
	checkEvery            string
	lastCheckStartTime    time.Time
	lastCheckEndTime      time.Time
	checkSetupError       error
	checkError            error
	uniqueVersionHistory  bool
}

func (t *resourceType) ID() int                       { return t.id }
func (t *resourceType) TeamID() int                   { return t.teamID }
func (t *resourceType) TeamName() string              { return t.teamName }
func (t *resourceType) Name() string                  { return t.name }
func (t *resourceType) Type() string                  { return t.type_ }
func (t *resourceType) Privileged() bool              { return t.privileged }
func (t *resourceType) CheckEvery() string            { return t.checkEvery }
func (t *resourceType) CheckTimeout() string          { return "" }
func (r *resourceType) LastCheckStartTime() time.Time { return r.lastCheckStartTime }
func (r *resourceType) LastCheckEndTime() time.Time   { return r.lastCheckEndTime }
func (t *resourceType) Source() atc.Source            { return t.source }
func (t *resourceType) Params() atc.Params            { return t.params }
func (t *resourceType) Tags() atc.Tags                { return t.tags }
func (t *resourceType) CheckSetupError() error        { return t.checkSetupError }
func (t *resourceType) CheckError() error             { return t.checkError }
func (t *resourceType) UniqueVersionHistory() bool    { return t.uniqueVersionHistory }
func (t *resourceType) ResourceConfigScopeID() int    { return t.resourceConfigScopeID }

func (t *resourceType) Version() atc.Version              { return t.version }
func (t *resourceType) CurrentPinnedVersion() atc.Version { return nil }

func (t *resourceType) HasWebhook() bool {
	return false
}

func newEmptyResourceType(conn Conn, lockFactory lock.LockFactory) *resourceType {
	return &resourceType{pipelineRef: pipelineRef{conn: conn, lockFactory: lockFactory}}
}

func (t *resourceType) Reload() (bool, error) {
	row := resourceTypesQuery.Where(sq.Eq{"r.id": t.id}).RunWith(t.conn).QueryRow()

	err := scanResourceType(t, row)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (t *resourceType) SetResourceConfig(source atc.Source, resourceTypes atc.VersionedResourceTypes) (ResourceConfigScope, error) {
	resourceConfigDescriptor, err := constructResourceConfigDescriptor(t.type_, source, resourceTypes)
	if err != nil {
		return nil, err
	}

	tx, err := t.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	resourceConfig, err := resourceConfigDescriptor.findOrCreate(tx, t.lockFactory, t.conn)
	if err != nil {
		return nil, err
	}

	_, err = psql.Update("resource_types").
		Set("resource_config_id", resourceConfig.ID()).
		Where(sq.Eq{
			"id": t.id,
		}).
		RunWith(tx).
		Exec()
	if err != nil {
		return nil, err
	}

	// A nil value is passed into the Resource object parameter because we always want resource type versions to be shared
	resourceConfigScope, err := findOrCreateResourceConfigScope(tx, t.conn, t.lockFactory, resourceConfig, nil, t.type_, resourceTypes)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return resourceConfigScope, nil
}

func (t *resourceType) SetCheckSetupError(cause error) error {
	var err error

	if cause == nil {
		_, err = psql.Update("resource_types").
			Set("check_error", nil).
			Where(sq.Eq{"id": t.id}).
			RunWith(t.conn).
			Exec()
	} else {
		_, err = psql.Update("resource_types").
			Set("check_error", cause.Error()).
			Where(sq.Eq{"id": t.id}).
			RunWith(t.conn).
			Exec()
	}

	return err
}

func scanResourceType(t *resourceType, row scannable) error {
	var (
		configJSON                                   sql.NullString
		checkErr, rcsCheckErr, rcsID, version, nonce sql.NullString
		lastCheckStartTime, lastCheckEndTime         pq.NullTime
		pipelineInstanceVars                         sql.NullString
	)

	err := row.Scan(&t.id, &t.pipelineID, &t.name, &t.type_, &configJSON, &version, &nonce, &checkErr, &t.pipelineName, &pipelineInstanceVars, &t.teamID, &t.teamName, &rcsID, &rcsCheckErr, &lastCheckStartTime, &lastCheckEndTime)
	if err != nil {
		return err
	}

	t.lastCheckStartTime = lastCheckStartTime.Time
	t.lastCheckEndTime = lastCheckEndTime.Time

	if version.Valid {
		err = json.Unmarshal([]byte(version.String), &t.version)
		if err != nil {
			return err
		}
	}

	es := t.conn.EncryptionStrategy()

	var noncense *string
	if nonce.Valid {
		noncense = &nonce.String
	}

	var config atc.ResourceType
	if configJSON.Valid {
		decryptedConfig, err := es.Decrypt(configJSON.String, noncense)
		if err != nil {
			return err
		}

		err = json.Unmarshal(decryptedConfig, &config)
		if err != nil {
			return err
		}
	} else {
		config = atc.ResourceType{}
	}

	t.source = config.Source
	t.params = config.Params
	t.privileged = config.Privileged
	t.tags = config.Tags
	t.checkEvery = config.CheckEvery
	t.uniqueVersionHistory = config.UniqueVersionHistory

	if checkErr.Valid {
		t.checkSetupError = errors.New(checkErr.String)
	}

	if rcsID.Valid {
		t.resourceConfigScopeID, err = strconv.Atoi(rcsID.String)
		if err != nil {
			return err
		}
	}

	if rcsCheckErr.Valid {
		t.checkError = errors.New(rcsCheckErr.String)
	}

	if pipelineInstanceVars.Valid {
		err = json.Unmarshal([]byte(pipelineInstanceVars.String), &t.pipelineInstanceVars)
		if err != nil {
			return err
		}
	}

	return nil
}
