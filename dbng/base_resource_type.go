package dbng

import (
	"database/sql"
	"errors"

	sq "github.com/Masterminds/squirrel"
	"github.com/lib/pq"
)

var ErrBaseResourceTypeAlreadyExists = errors.New("cache already exists")
var ErrBaseResourceTypeResourceTypeVolumeDisappeared = errors.New("cache resource type volume disappeared")

type BaseResourceType struct {
	Name    string
	Image   string
	Version string
}

type UsedBaseResourceType struct {
	ID int
}

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
