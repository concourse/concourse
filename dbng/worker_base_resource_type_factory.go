package dbng

import (
	"database/sql"

	sq "github.com/Masterminds/squirrel"
)

//go:generate counterfeiter . WorkerBaseResourceTypeFactory

type WorkerBaseResourceTypeFactory interface {
	Find(name string, worker Worker) (*UsedWorkerBaseResourceType, bool, error)
}

type workerBaseResourceTypeFactory struct {
	conn Conn
}

func NewWorkerBaseResourceTypeFactory(conn Conn) WorkerBaseResourceTypeFactory {
	return &workerBaseResourceTypeFactory{
		conn: conn,
	}
}

func (f *workerBaseResourceTypeFactory) Find(name string, worker Worker) (*UsedWorkerBaseResourceType, bool, error) {
	tx, err := f.conn.Begin()
	if err != nil {
		return nil, false, err
	}

	defer tx.Rollback()

	var id int
	var version string
	err = psql.Select("wbrt.id, wbrt.version").
		From("worker_base_resource_types wbrt").
		LeftJoin("base_resource_types brt ON brt.id = wbrt.base_resource_type_id").
		Where(sq.Eq{
			"brt.name":         name,
			"wbrt.worker_name": worker.Name(),
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

	err = tx.Commit()
	if err != nil {
		return nil, false, err
	}

	return &UsedWorkerBaseResourceType{
		ID:      id,
		Name:    name,
		Version: version,
		Worker:  worker,
	}, true, nil
}
