package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
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
	Space() atc.Space

	Version() (atc.Version, error)

	SetResourceConfig(int) error
	SetCheckError(error) error

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
				Name:       t.Name(),
				Type:       t.Type(),
				Space:      t.Space(),
				Source:     t.Source(),
				Privileged: t.Privileged(),
				CheckEvery: t.CheckEvery(),
				Tags:       t.Tags(),
				Params:     t.Params(),
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
			Name:       r.Name(),
			Type:       r.Type(),
			Space:      r.Space(),
			Source:     r.Source(),
			Privileged: r.Privileged(),
			CheckEvery: r.CheckEvery(),
			Tags:       r.Tags(),
			Params:     r.Params(),
		})
	}

	return configs
}

var resourceTypesQuery = psql.Select("r.id, r.name, r.type, r.config, r.space, r.nonce, r.check_error, c.check_error").
	From("resource_types r").
	LeftJoin("resource_configs c ON r.resource_config_id = c.id").
	Where(sq.Eq{"r.active": true})

type resourceType struct {
	id                       int
	name                     string
	type_                    string
	privileged               bool
	source                   atc.Source
	params                   atc.Params
	tags                     atc.Tags
	space                    atc.Space
	checkEvery               string
	checkError               error
	resourceConfigCheckError error

	conn Conn
}

func (t *resourceType) ID() int                         { return t.id }
func (t *resourceType) Name() string                    { return t.name }
func (t *resourceType) Type() string                    { return t.type_ }
func (t *resourceType) Space() atc.Space                { return t.space }
func (t *resourceType) Privileged() bool                { return t.privileged }
func (t *resourceType) CheckEvery() string              { return t.checkEvery }
func (t *resourceType) Source() atc.Source              { return t.source }
func (t *resourceType) Params() atc.Params              { return t.params }
func (t *resourceType) Tags() atc.Tags                  { return t.tags }
func (t *resourceType) CheckError() error               { return t.checkError }
func (t *resourceType) ResourceConfigCheckError() error { return t.resourceConfigCheckError }

func (t *resourceType) Version() (atc.Version, error) {
	var version atc.Version
	var versionBlob sql.NullString

	if t.space != "" {
		err := psql.Select("rv.version").
			From("resource_versions rv, spaces s, resource_types rt").
			Where(sq.Expr("rv.space_id = s.id")).
			Where(sq.Expr("rt.resource_config_id = s.resource_config_id")).
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
			Join("spaces s ON rc.default_space = s.name AND rc.id = s.resource_config_id").
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

func (t *resourceType) SetResourceConfig(resourceConfigID int) error {
	_, err := psql.Update("resource_types").
		Set("resource_config_id", resourceConfigID).
		Where(sq.Eq{
			"id": t.id,
		}).
		RunWith(t.conn).
		Exec()

	return err
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
		configJSON                         []byte
		checkErr, rcCheckErr, nonce, space sql.NullString
	)

	err := row.Scan(&t.id, &t.name, &t.type_, &configJSON, &space, &nonce, &checkErr, &rcCheckErr)
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

	if checkErr.Valid {
		t.checkError = errors.New(checkErr.String)
	}

	if rcCheckErr.Valid {
		t.resourceConfigCheckError = errors.New(rcCheckErr.String)
	}

	return nil
}
