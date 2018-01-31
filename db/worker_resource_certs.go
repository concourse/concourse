package db

import (
	"database/sql"

	sq "github.com/Masterminds/squirrel"
)

type WorkerResourceCerts struct {
	WorkerName string
	CertsPath  string
}

type UsedWorkerResourceCerts struct {
	ID int
}

func (workerResourceCerts WorkerResourceCerts) Find(runner sq.Runner) (*UsedWorkerResourceCerts, bool, error) {
	var id int
	err := psql.Select("id").
		From("worker_resource_certs").
		Where(sq.Eq{
			"worker_name": workerResourceCerts.WorkerName,
			"certs_path":  workerResourceCerts.CertsPath,
		}).
		RunWith(runner).
		QueryRow().
		Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}

		return nil, false, err
	}

	return &UsedWorkerResourceCerts{ID: id}, true, nil
}

func (workerResourceCerts WorkerResourceCerts) Create(tx Tx) (*UsedWorkerResourceCerts, error) {
	var id int
	err := psql.Insert("worker_resource_certs").
		Columns(
			"worker_name",
			"certs_path",
		).
		Values(
			workerResourceCerts.WorkerName,
			workerResourceCerts.CertsPath,
		).
		Suffix("RETURNING id").
		RunWith(tx).
		QueryRow().
		Scan(&id)
	if err != nil {
		return nil, err
	}

	return &UsedWorkerResourceCerts{
		ID: id,
	}, nil
}
