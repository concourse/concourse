package dbng

import (
	"database/sql"
	"errors"

	sq "github.com/Masterminds/squirrel"
	"github.com/lib/pq"
)

var ErrBaseResourceTypeAlreadyExists = errors.New("cache already exists")

// BaseResourceType represents a resource type provided by workers.
//
// It is created via worker registration. All creates are upserts.
//
// It is removed by gc.BaseResourceTypeCollector, once there are no references
// to it from worker_base_resource_types.
type BaseResourceType struct {
	Name    string // The name of the type, e.g. 'git'.
	Image   string // The path to the image, e.g. '/opt/concourse/resources/git'.
	Version string // The version of the image, e.g. a SHA of the rootfs.
}

// UsedBaseResourceType is created whenever a ResourceConfig is used, either
// for a build, a resource in the pipeline, or a resource type in the pipeline.
//
// So long as the UsedBaseResourceType's ID is referenced by a ResourceConfig
// that is in use, this guarantees that the BaseResourceType will not be
// removed. That is to say that its "Use" is vicarious.
type UsedBaseResourceType struct {
	ID int // The ID of the BaseResourceType.
}

// FindOrCreate looks for an existing BaseResourceType and creates it if it
// doesn't exist. It returns a UsedBaseResourceType.
//
// Note that if the BaseResourceType already existed, there's a chance that it
// will be garbage-collected before the referencing ResourceConfig can be
// created and used.
//
// This method can return ErrBaseResourceTypeAlreadyExists if two concurrent
// FindOrCreates clashed. The caller should retry from the start of the
// transaction.
func (brt BaseResourceType) FindOrCreate(tx Tx) (*UsedBaseResourceType, error) {
	ubrt, found, err := brt.find(tx)
	if err != nil {
		return nil, err
	}

	if found {
		return ubrt, nil
	}

	return brt.create(tx)
}

func (brt BaseResourceType) find(tx Tx) (*UsedBaseResourceType, bool, error) {
	var id int
	err := psql.Select("id").From("base_resource_types").Where(sq.Eq{
		"name":    brt.Name,
		"image":   brt.Image,
		"version": brt.Version,
	}).RunWith(tx).QueryRow().Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}

		return nil, false, err
	}

	return &UsedBaseResourceType{ID: id}, true, nil
}

func (brt BaseResourceType) create(tx Tx) (*UsedBaseResourceType, error) {
	var id int
	err := psql.Insert("base_resource_types").
		Columns(
			"name",
			"image",
			"version",
		).
		Values(
			brt.Name,
			brt.Image,
			brt.Version,
		).
		Suffix("RETURNING id").
		RunWith(tx).
		QueryRow().
		Scan(&id)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "unique_violation" {
			return nil, ErrBaseResourceTypeAlreadyExists
		}

		return nil, err
	}

	return &UsedBaseResourceType{ID: id}, nil
}
