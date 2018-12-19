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
	CheckError() error
	ResourceConfigCheckError() error

	SetResourceConfig(lager.Logger, atc.Source, creds.VersionedResourceTypes) (ResourceConfig, error)
	SetCheckError(error) error

	Version() atc.Version

	Reload() (bool, error)
}

type ResourceTypes []ResourceType

func (resourceTypes ResourceTypes) Deserialize() atc.VersionedResourceTypes {
	var versionedResourceTypes atc.VersionedResourceTypes

	for _, t := range resourceTypes {
		versionedResourceTypes = append(versionedResourceTypes, atc.VersionedResourceType{
			ResourceType: atc.ResourceType{
				Name:       t.Name(),
				Type:       t.Type(),
				Source:     t.Source(),
				Privileged: t.Privileged(),
				CheckEvery: t.CheckEvery(),
				Tags:       t.Tags(),
				Params:     t.Params(),
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
			Name:       r.Name(),
			Type:       r.Type(),
			Source:     r.Source(),
			Privileged: r.Privileged(),
			CheckEvery: r.CheckEvery(),
			Tags:       r.Tags(),
			Params:     r.Params(),
		})
	}

	return configs
}

var resourceTypesQuery = psql.Select("r.id, r.name, r.type, r.config, rcv.version, r.nonce, r.check_error, c.check_error").
	From("resource_types r").
	LeftJoin("resource_configs c ON r.resource_config_id = c.id").
	LeftJoin(`LATERAL (
		SELECT rcv.*
		FROM resource_config_versions rcv
		WHERE rcv.resource_config_id = c.id AND rcv.check_order != 0
		ORDER BY rcv.check_order DESC
		LIMIT 1
	) AS rcv ON true`).
	Where(sq.Eq{"r.active": true})

type resourceType struct {
	id                       int
	name                     string
	type_                    string
	privileged               bool
	source                   atc.Source
	params                   atc.Params
	tags                     atc.Tags
	version                  atc.Version
	checkEvery               string
	checkError               error
	resourceConfigCheckError error

	conn        Conn
	lockFactory lock.LockFactory
}

func (t *resourceType) ID() int                         { return t.id }
func (t *resourceType) Name() string                    { return t.name }
func (t *resourceType) Type() string                    { return t.type_ }
func (t *resourceType) Privileged() bool                { return t.privileged }
func (t *resourceType) CheckEvery() string              { return t.checkEvery }
func (t *resourceType) Source() atc.Source              { return t.source }
func (t *resourceType) Params() atc.Params              { return t.params }
func (t *resourceType) Tags() atc.Tags                  { return t.tags }
func (t *resourceType) CheckError() error               { return t.checkError }
func (t *resourceType) ResourceConfigCheckError() error { return t.resourceConfigCheckError }

func (t *resourceType) Version() atc.Version { return t.version }

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

func (t *resourceType) SetResourceConfig(logger lager.Logger, source atc.Source, resourceTypes creds.VersionedResourceTypes) (ResourceConfig, error) {
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

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return resourceConfig, err
}

func (t *resourceType) SetCheckError(cause error) error {
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
		configJSON                           []byte
		checkErr, rcCheckErr, version, nonce sql.NullString
	)

	err := row.Scan(&t.id, &t.name, &t.type_, &configJSON, &version, &nonce, &checkErr, &rcCheckErr)
	if err != nil {
		return err
	}

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

	if checkErr.Valid {
		t.checkError = errors.New(checkErr.String)
	}

	if rcCheckErr.Valid {
		t.resourceConfigCheckError = errors.New(rcCheckErr.String)
	}

	return nil
}
