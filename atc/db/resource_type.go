package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"code.cloudfoundry.org/lager"
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db/lock"
)

type ResourceTypeNotFoundError struct {
	Name string
}

func (e ResourceTypeNotFoundError) Error() string {
	return fmt.Sprintf("resource type not found: %s", e.Name)
}

//go:generate counterfeiter . ResourceType

type ResourceType interface {
	ID() int
	Name() string
	Type() string
	Privileged() bool
	Source() atc.Source
	Params() atc.Params
	Tags() atc.Tags
	CheckEvery() string
	CheckSetupError() error
	CheckError() error
	Space() atc.Space

	Version() (atc.Version, error)
	UniqueVersionHistory() bool

	SetResourceConfig(lager.Logger, atc.Source, creds.VersionedResourceTypes) (ResourceConfigScope, error)
	SetCheckSetupError(error) error

	Reload() (bool, error)
}

type ResourceTypes []ResourceType

func (resourceTypes ResourceTypes) Deserialize() (atc.VersionedResourceTypes, error) {
	var versionedResourceTypes atc.VersionedResourceTypes

	for _, t := range resourceTypes {
		version, err := t.Version()
		if err != nil {
			return nil, err
		}

		versionedResourceTypes = append(versionedResourceTypes, atc.VersionedResourceType{
			ResourceType: atc.ResourceType{
				Name:                 t.Name(),
				Type:                 t.Type(),
				Space:                t.Space(),
				Source:               t.Source(),
				Privileged:           t.Privileged(),
				CheckEvery:           t.CheckEvery(),
				Tags:                 t.Tags(),
				Params:               t.Params(),
				UniqueVersionHistory: t.UniqueVersionHistory(),
			},
			Version: version,
		})
	}

	return versionedResourceTypes, nil
}

func (resourceTypes ResourceTypes) Configs() atc.ResourceTypes {
	var configs atc.ResourceTypes

	for _, r := range resourceTypes {
		configs = append(configs, atc.ResourceType{
			Name:                 r.Name(),
			Type:                 r.Type(),
			Space:                r.Space(),
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

var resourceTypesQuery = psql.Select("r.id, r.name, r.type, r.config, r.space, r.nonce, r.check_error, s.check_error").
	From("resource_types r").
	LeftJoin("resource_config_scopes s ON r.resource_config_id = s.resource_config_id").
	Where(sq.Eq{"r.active": true})

type resourceType struct {
	id                   int
	name                 string
	type_                string
	privileged           bool
	source               atc.Source
	params               atc.Params
	tags                 atc.Tags
	space                atc.Space
	checkEvery           string
	checkSetupError      error
	checkError           error
	uniqueVersionHistory bool

	conn        Conn
	lockFactory lock.LockFactory
}

func (t *resourceType) ID() int                    { return t.id }
func (t *resourceType) Name() string               { return t.name }
func (t *resourceType) Type() string               { return t.type_ }
func (t *resourceType) Space() atc.Space           { return t.space }
func (t *resourceType) Privileged() bool           { return t.privileged }
func (t *resourceType) CheckEvery() string         { return t.checkEvery }
func (t *resourceType) Source() atc.Source         { return t.source }
func (t *resourceType) Params() atc.Params         { return t.params }
func (t *resourceType) Tags() atc.Tags             { return t.tags }
func (t *resourceType) CheckSetupError() error     { return t.checkSetupError }
func (t *resourceType) CheckError() error          { return t.checkError }
func (t *resourceType) UniqueVersionHistory() bool { return t.uniqueVersionHistory }

func (t *resourceType) Version() (atc.Version, error) {
	var version atc.Version
	var versionBlob sql.NullString

	if t.space != "" {
		err := psql.Select("rv.version").
			From("resource_versions rv, spaces s, resource_config_scopes rs, resource_types rt").
			Where(sq.Expr("rv.space_id = s.id")).
			Where(sq.Expr("s.resource_config_scope_id = rs.id")).
			Where(sq.Expr("rs.resource_config_id = rt.resource_config_id")).
			Where(sq.Eq{
				"s.name": t.space,
				"rt.id":  t.id,
			}).
			Where(sq.NotEq{
				"rv.check_order": 0,
			}).
			OrderBy("rv.check_order DESC").
			Limit(1).
			RunWith(t.conn).
			QueryRow().
			Scan(&versionBlob)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
	} else {
		err := psql.Select("rv.version").
			From("resource_types rt").
			Join("resource_configs rc ON rt.resource_config_id = rc.id").
			Join("resource_config_scopes rs ON rc.id = rs.resource_config_id").
			Join("spaces s ON rs.default_space = s.name AND rs.id = s.resource_config_scope_id").
			Join("resource_versions rv ON s.latest_resource_version_id = rv.id").
			Where(sq.Eq{
				"rt.id": t.id,
			}).
			RunWith(t.conn).
			QueryRow().
			Scan(&versionBlob)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
	}

	if versionBlob.Valid {
		err := json.Unmarshal([]byte(versionBlob.String), &version)
		if err != nil {
			return nil, err
		}
	}

	return version, nil
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

func (t *resourceType) SetResourceConfig(logger lager.Logger, source atc.Source, resourceTypes creds.VersionedResourceTypes) (ResourceConfigScope, error) {
	resourceConfigDescriptor, err := constructResourceConfigDescriptor(t.type_, source, resourceTypes)
	if err != nil {
		return nil, err
	}

	tx, err := t.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	resourceConfig, err := resourceConfigDescriptor.findOrCreate(logger, tx, t.lockFactory, t.conn)
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
	resourceConfigScope, err := findOrCreateResourceConfigScope(tx, t.conn, t.lockFactory, resourceConfig, nil, resourceTypes)
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
		configJSON                          []byte
		checkErr, rcsCheckErr, nonce, space sql.NullString
	)

	err := row.Scan(&t.id, &t.name, &t.type_, &configJSON, &space, &nonce, &checkErr, &rcsCheckErr)
	if err != nil {
		return err
	}

	if space.Valid {
		t.space = atc.Space(space.String)
	}

	es := t.conn.EncryptionStrategy()

	var noncense *string
	if nonce.Valid {
		noncense = &nonce.String
	}

	decryptedConfig, err := es.Decrypt(string(configJSON), noncense)
	if err != nil {
		return err
	}

	var config atc.ResourceType
	err = json.Unmarshal(decryptedConfig, &config)
	if err != nil {
		return err
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

	if rcsCheckErr.Valid {
		t.checkError = errors.New(rcsCheckErr.String)
	}

	return nil
}
