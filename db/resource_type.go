package db

import (
	"database/sql"
	"encoding/json"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
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

	SetResourceConfig(int) error

	Version() atc.Version
	SaveVersion(atc.Version) error

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

var resourceTypesQuery = psql.Select("id, name, type, config, version, nonce").
	From("resource_types").
	Where(sq.Eq{"active": true})

type resourceType struct {
	id         int
	name       string
	type_      string
	privileged bool
	source     atc.Source
	params     atc.Params
	tags       atc.Tags
	version    atc.Version
	checkEvery string

	conn Conn
}

func (t *resourceType) ID() int            { return t.id }
func (t *resourceType) Name() string       { return t.name }
func (t *resourceType) Type() string       { return t.type_ }
func (t *resourceType) Privileged() bool   { return t.privileged }
func (t *resourceType) CheckEvery() string { return t.checkEvery }
func (t *resourceType) Source() atc.Source { return t.source }
func (t *resourceType) Params() atc.Params { return t.params }
func (r *resourceType) Tags() atc.Tags     { return r.tags }

func (t *resourceType) Version() atc.Version { return t.version }
func (t *resourceType) SaveVersion(version atc.Version) error {
	versionJSON, err := json.Marshal(version)
	if err != nil {
		return err
	}

	result, err := psql.Update("resource_types").
		Where(sq.Eq{"id": t.id}).
		Set("version", versionJSON).
		RunWith(t.conn).
		Exec()
	if err != nil {
		return err
	}

	num, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if num == 0 {
		return ResourceTypeNotFoundError{t.name}
	}

	return nil
}

func (t *resourceType) Reload() (bool, error) {
	row := resourceTypesQuery.Where(sq.Eq{"id": t.id}).RunWith(t.conn).QueryRow()

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

func scanResourceType(t *resourceType, row scannable) error {
	var (
		configJSON     []byte
		version, nonce sql.NullString
	)

	err := row.Scan(&t.id, &t.name, &t.type_, &configJSON, &version, &nonce)
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

	return nil
}
