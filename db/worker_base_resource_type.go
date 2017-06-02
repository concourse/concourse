package db

import (
	"database/sql"

	sq "github.com/Masterminds/squirrel"
)

type WorkerBaseResourceType struct {
	Name       string
	WorkerName string
}

type UsedWorkerBaseResourceType struct {
	ID      int
	Name    string
	Version string

	Worker Worker
}

func (workerBaseResourceType WorkerBaseResourceType) Find(runner sq.Runner) (*UsedWorkerBaseResourceType, bool, error) {
	var id int
	var version string
	var sqWorkerAddress sql.NullString
	var sqWorkerBaggageclaimURL sql.NullString
	err := psql.Select("wbrt.id, wbrt.version, w.addr, w.baggageclaim_url").
		From("worker_base_resource_types wbrt").
		LeftJoin("base_resource_types brt ON brt.id = wbrt.base_resource_type_id").
		LeftJoin("workers w ON w.name = wbrt.worker_name").
		Where(sq.Eq{
			"brt.name":         workerBaseResourceType.Name,
			"wbrt.worker_name": workerBaseResourceType.WorkerName,
		}).
		RunWith(runner).
		QueryRow().
		Scan(&id, &version, &sqWorkerAddress, &sqWorkerBaggageclaimURL)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}

		return nil, false, err
	}

	var workerBaggageclaimURL string
	if sqWorkerBaggageclaimURL.Valid {
		workerBaggageclaimURL = sqWorkerBaggageclaimURL.String
	}

	var workerAddress string
	if sqWorkerAddress.Valid {
		workerAddress = sqWorkerAddress.String
	}

	return &UsedWorkerBaseResourceType{
		ID:      id,
		Name:    workerBaseResourceType.Name,
		Version: version,
		Worker: &worker{
			name:            workerBaseResourceType.WorkerName,
			gardenAddr:      &workerAddress,
			baggageclaimURL: &workerBaggageclaimURL,
		},
	}, true, nil
}
