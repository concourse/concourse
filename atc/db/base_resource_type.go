package db

import (
	"database/sql"

	sq "github.com/Masterminds/squirrel"
)

// BaseResourceType represents a resource type provided by workers.
//
// It is created via worker registration. All creates are upserts.
//
// It is removed by gc.BaseResourceTypeCollector, once there are no references
// to it from worker_base_resource_types.
type BaseResourceType struct {
	Name string // The name of the type, e.g. 'git'.
}

// UsedBaseResourceType is created whenever a ResourceConfig is used, either
// for a build, a resource in the pipeline, or a resource type in the pipeline.
//
// So long as the UsedBaseResourceType's ID is referenced by a ResourceConfig
// that is in use, this guarantees that the BaseResourceType will not be
// removed. That is to say that its "Use" is vicarious.
type UsedBaseResourceType struct {
	ID   int // The ID of the BaseResourceType.
	Name string
}

// FindOrCreate looks for an existing BaseResourceType and creates it if it
// doesn't exist. It returns a UsedBaseResourceType.
func (brt BaseResourceType) FindOrCreate(tx Tx) (*UsedBaseResourceType, error) {
	ubrt, found, err := brt.Find(tx)
	if err != nil {
		return nil, err
	}

	if found {
		return ubrt, nil
	}

	return brt.create(tx)
}

func (brt BaseResourceType) Find(runner sq.Runner) (*UsedBaseResourceType, bool, error) {
	var id int
	err := psql.Select("id").
		From("base_resource_types").
		Where(sq.Eq{"name": brt.Name}).
		Suffix("FOR SHARE").
		RunWith(runner).
		QueryRow().
		Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}

		return nil, false, err
	}

	return &UsedBaseResourceType{ID: id, Name: brt.Name}, true, nil
}

func (brt BaseResourceType) create(tx Tx) (*UsedBaseResourceType, error) {
	var id int
	err := psql.Insert("base_resource_types").
		Columns("name").
		Values(brt.Name).
		Suffix(`
			ON CONFLICT (name) DO UPDATE SET name = ?
			RETURNING id
		`, brt.Name).
		RunWith(tx).
		QueryRow().
		Scan(&id)
	if err != nil {
		return nil, err
	}

	return &UsedBaseResourceType{ID: id, Name: brt.Name}, nil
}
