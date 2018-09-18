package db

import (
	"database/sql"

	sq "github.com/Masterminds/squirrel"
)

type WorkerResourceType struct {
	Worker  Worker
	Image   string // The path to the image, e.g. '/opt/concourse/resources/git'.
	Version string // The version of the image, e.g. a SHA of the rootfs.

	BaseResourceType *BaseResourceType
}

type UsedWorkerResourceType struct {
	ID int

	Worker Worker

	UsedBaseResourceType *UsedBaseResourceType
}

func (wrt WorkerResourceType) FindOrCreate(tx Tx) (*UsedWorkerResourceType, error) {
	usedBaseResourceType, err := wrt.BaseResourceType.FindOrCreate(tx)
	if err != nil {
		return nil, err
	}

	uwrt, found, err := wrt.find(tx, usedBaseResourceType)
	if err != nil {
		return nil, err
	}

	if found {
		return uwrt, nil
	}

	return wrt.create(tx, usedBaseResourceType)
}

func (wrt WorkerResourceType) find(tx Tx, usedBaseResourceType *UsedBaseResourceType) (*UsedWorkerResourceType, bool, error) {
	var (
		workerName string
		id         int
	)
	err := psql.Select("id", "worker_name").
		From("worker_base_resource_types").
		Where(sq.Eq{
			"worker_name":           wrt.Worker.Name(),
			"base_resource_type_id": usedBaseResourceType.ID,
			"image":                 wrt.Image,
			"version":               wrt.Version,
		}).
		Suffix("FOR SHARE").
		RunWith(tx).
		QueryRow().
		Scan(&id, &workerName)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	return &UsedWorkerResourceType{
		ID:                   id,
		Worker:               wrt.Worker,
		UsedBaseResourceType: usedBaseResourceType,
	}, true, nil
}

func (wrt WorkerResourceType) create(tx Tx, usedBaseResourceType *UsedBaseResourceType) (*UsedWorkerResourceType, error) {
	var id int
	err := psql.Insert("worker_base_resource_types").
		Columns(
			"worker_name",
			"base_resource_type_id",
			"image",
			"version",
		).
		Values(
			wrt.Worker.Name(),
			usedBaseResourceType.ID,
			wrt.Image,
			wrt.Version,
		).
		Suffix(`
			ON CONFLICT (worker_name, base_resource_type_id) DO UPDATE SET
				image = ?,
				version = ?
			RETURNING id
		`, wrt.Image, wrt.Version).
		RunWith(tx).
		QueryRow().
		Scan(&id)
	if err != nil {
		return nil, err
	}

	return &UsedWorkerResourceType{
		ID:                   id,
		Worker:               wrt.Worker,
		UsedBaseResourceType: usedBaseResourceType,
	}, nil
}
