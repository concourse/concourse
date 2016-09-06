package dbng

import (
	"database/sql"
	"encoding/json"
	"errors"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/lib/pq"
)

// TODO: rename to something Resource related?

type Cache struct {
	ResourceTypeVolume *InitializedVolume
	Source             atc.Source
	Params             atc.Params
	Version            atc.Version
}

func (cache Cache) Lookup(tx Tx) (int, bool, error) {
	var id int
	err := psql.Select("id").From("caches").Where(sq.Eq{
		"resource_type_volume_id": cache.ResourceTypeVolume.ID,
		"source_hash":             cache.hash(cache.Source),
		"params_hash":             cache.hash(cache.Params),
		"version":                 cache.version(),
	}).RunWith(tx).QueryRow().Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, false, nil
		}

		return 0, false, err
	}

	return id, true, nil
}

var ErrCacheAlreadyExists = errors.New("cache already exists")
var ErrCacheResourceTypeVolumeDisappeared = errors.New("cache resource type volume disappeared")

func (cache Cache) Create(tx Tx) (int, error) {
	var id int
	err := psql.Insert("caches").
		Columns(
			"resource_type_volume_id",
			"resource_type_volume_state",
			"source_hash",
			"params_hash",
			"version",
		).
		Values(
			cache.ResourceTypeVolume.ID,
			VolumeStateInitialized,
			cache.hash(cache.Source),
			cache.hash(cache.Params),
			cache.version(),
		).
		Suffix("RETURNING id").
		RunWith(tx).
		QueryRow().
		Scan(&id)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "unique_violation" {
			return 0, ErrCacheAlreadyExists
		}

		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "foreign_key_violation" {
			return 0, ErrCacheResourceTypeVolumeDisappeared
		}

		return 0, err
	}

	return id, nil
}

func (Cache) Destroy(Tx) (bool, error) {
	// return false for constraint violations, propagate any other errs
	return false, nil
}

func (Cache) hash(prop interface{}) string {
	j, _ := json.Marshal(prop)
	return string(j) // TODO: actually hash
}

func (cache Cache) version() string {
	j, _ := json.Marshal(cache.Version)
	return string(j)
}
