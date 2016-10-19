package dbng

import (
	"database/sql"
	"encoding/json"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
)

type ResourceType struct {
	atc.ResourceType

	Version  atc.Version
	Pipeline *Pipeline
}

type UsedResourceType struct {
	ID      int
	Version atc.Version
}

func (resourceType ResourceType) Find(tx Tx) (*UsedResourceType, bool, error) {
	var id int
	var version sql.NullString

	err := psql.Select("id", "version").
		From("resource_types").
		Where(sq.Eq{
			"pipeline_id": resourceType.Pipeline.ID,
			"name":        resourceType.Name,
		}).
		RunWith(tx).
		QueryRow().
		Scan(&id, &version)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}

		return nil, false, err
	}

	urt := &UsedResourceType{
		ID: id,
	}

	if version.Valid {
		err = json.Unmarshal([]byte(version.String), urt.Version)
		if err != nil {
			return nil, false, err
		}
	}

	return urt, true, nil
}

func (resourceType ResourceType) Create(tx Tx) (*UsedResourceType, error) {
	versionString, err := json.Marshal(resourceType.Version)
	if err != nil {
		return nil, err
	}

	configPayload, err := json.Marshal(resourceType.ResourceType)
	if err != nil {
		return nil, err
	}

	var id int
	err = psql.Insert("resource_types").
		Columns("pipeline_id", "name", "type", "version", "config").
		Values(resourceType.Pipeline.ID, resourceType.Name, resourceType.Type, versionString, configPayload).
		Suffix("RETURNING id").
		RunWith(tx).
		QueryRow().
		Scan(&id)
	if err != nil {
		// TODO: explicitly handle fkey constraint
		return nil, err
	}

	return &UsedResourceType{
		ID:      id,
		Version: resourceType.Version,
	}, nil
}
