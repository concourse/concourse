package db

import (
	"database/sql"
	"github.com/patrickmn/go-cache"
	"time"

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
	ID                   int    // The ID of the BaseResourceType.
	Name                 string // The name of the type, e.g. 'git'.
	UniqueVersionHistory bool   // If set to true, will create unique version histories for each of the resources using this base resource type
}

const baseResourceTypesCacheExpiration = 10 * time.Minute

// baseResourceTypesCache caches base resource types in memory as base resource
// types are steady. In theory, they should never change unless an upgrade happens,
// because Concourse doesn't provide a way to upgrade base resource types on
// workers. But the cache data will have a expiration to ensure cache fresh.
var baseResourceTypesCache = cache.New(0, 1*time.Hour)

// CleanupBaseResourceTypesCache should only be used in unit tests' BeforeEach.
func CleanupBaseResourceTypesCache() {
	baseResourceTypesCache = cache.New(0, 1*time.Hour)
}

// FindOrCreate looks for an existing BaseResourceType and creates it if it
// doesn't exist. It returns a UsedBaseResourceType.
func (brt BaseResourceType) FindOrCreate(tx Tx, unique bool) (*UsedBaseResourceType, error) {
	ubrt, found, err := brt.Find(tx)
	if err != nil {
		return nil, err
	}

	if found && ubrt.UniqueVersionHistory == unique {
		return ubrt, nil
	}

	return brt.create(tx, unique)
}

func (brt BaseResourceType) Find(runner sq.Runner) (*UsedBaseResourceType, bool, error) {
	if ubrt, found := baseResourceTypesCache.Get(brt.Name); found {
		return ubrt.(*UsedBaseResourceType), true, nil
	}

	var id int
	var unique bool
	err := psql.Select("id, unique_version_history").
		From("base_resource_types").
		Where(sq.Eq{"name": brt.Name}).
		Suffix("FOR SHARE").
		RunWith(runner).
		QueryRow().
		Scan(&id, &unique)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}

		return nil, false, err
	}

	ubrt := UsedBaseResourceType{ID: id, Name: brt.Name, UniqueVersionHistory: unique}
	baseResourceTypesCache.Set(brt.Name, &ubrt, baseResourceTypesCacheExpiration)

	return &ubrt, true, nil
}

func (brt BaseResourceType) create(tx Tx, unique bool) (*UsedBaseResourceType, error) {
	var id int
	var savedUnique bool
	err := psql.Insert("base_resource_types").
		Columns("name", "unique_version_history").
		Values(brt.Name, unique).
		Suffix(`
			ON CONFLICT (name) DO UPDATE SET
				name = EXCLUDED.name,
				unique_version_history = EXCLUDED.unique_version_history OR base_resource_types.unique_version_history
			RETURNING id, unique_version_history
		`).
		RunWith(tx).
		QueryRow().
		Scan(&id, &savedUnique)
	if err != nil {
		return nil, err
	}

	ubrt := UsedBaseResourceType{ID: id, Name: brt.Name, UniqueVersionHistory: savedUnique}
	baseResourceTypesCache.Set(brt.Name, &ubrt, baseResourceTypesCacheExpiration)

	return &ubrt, nil
}
