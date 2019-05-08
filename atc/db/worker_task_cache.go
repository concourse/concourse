package db

import (
	"database/sql"

	sq "github.com/Masterminds/squirrel"
)

type WorkerTaskCache struct {
	WorkerName string
	TaskCache  UsedTaskCache
}

type UsedWorkerTaskCache struct {
	ID         int
	WorkerName string
}

func (wtc WorkerTaskCache) FindOrCreate(
	tx Tx,
) (*UsedWorkerTaskCache, error) {
	var id int
	err := psql.Select("id").
		From("worker_task_caches").
		Where(sq.Eq{
			"worker_name":   wtc.WorkerName,
			"task_cache_id": wtc.TaskCache.ID(),
		}).
		RunWith(tx).
		QueryRow().
		Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			err = psql.Insert("task_caches").
				Columns(
					"job_id",
					"step_name",
					"path",
				).
				Values(
					wtc.TaskCache.JobID(),
					wtc.TaskCache.StepName(),
					wtc.TaskCache.Path(),
				).
				Suffix(`
					ON CONFLICT (job_id, step_name, path) DO UPDATE SET
						job_id = ?
					RETURNING id
				`, wtc.TaskCache.JobID()).
				RunWith(tx).
				QueryRow().
				Scan(&id)
			if err != nil {
				return nil, err
			}

			err = psql.Insert("worker_task_caches").
				Columns(
					"worker_name",
					"task_cache_id",
				).
				Values(wtc.WorkerName, id).
				Suffix(`
					ON CONFLICT (worker_name, task_cache_id) DO UPDATE SET
						task_cache_id = ?
					RETURNING id
				`, id).
				RunWith(tx).
				QueryRow().
				Scan(&id)
			if err != nil {
				return nil, err
			}

			return &UsedWorkerTaskCache{
				ID:         id,
				WorkerName: wtc.WorkerName,
			}, nil
		}

		return nil, err
	}

	return &UsedWorkerTaskCache{
		ID:         id,
		WorkerName: wtc.WorkerName,
	}, nil
}

func (workerTaskCache WorkerTaskCache) Find(runner sq.Runner) (*UsedWorkerTaskCache, bool, error) {
	var id int
	err := psql.Select("id").
		From("worker_task_caches").
		Where(sq.Eq{
			"worker_name":   workerTaskCache.WorkerName,
			"task_cache_id": workerTaskCache.TaskCache.ID(),
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

	return &UsedWorkerTaskCache{
		ID:         id,
		WorkerName: workerTaskCache.WorkerName,
	}, true, nil
}
