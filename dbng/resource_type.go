package dbng

import (
	"database/sql"
	"encoding/json"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
)

type ErrResourceTypeNotFound struct {
	resourceTypeName string
}

func (e ErrResourceTypeNotFound) Error() string {
	return fmt.Sprintf("resource type not found %s", e.resourceTypeName)
}

type ResourceType struct {
	atc.ResourceType

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
		err = json.Unmarshal([]byte(version.String), &urt.Version)
		if err != nil {
			return nil, false, err
		}
	}

	return urt, true, nil
}

func (resourceType ResourceType) Create(tx Tx, version atc.Version) (*UsedResourceType, error) {
	configPayload, err := json.Marshal(resourceType.ResourceType)
	if err != nil {
		return nil, err
	}

	versionString, err := json.Marshal(version)
	if err != nil {
		return nil, err
	}

	columns := []string{"pipeline_id", "name", "type", "config", "version"}
	values := []interface{}{resourceType.Pipeline.ID, resourceType.Name, resourceType.Type, configPayload, versionString}

	var id int
	err = psql.Insert("resource_types").
		Columns(columns...).
		Values(values...).
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
		Version: version,
	}, nil
}
