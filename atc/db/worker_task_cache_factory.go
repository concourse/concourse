package db

import (
	"database/sql"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
)

type UsedWorkerTaskCache struct {
	ID         int
	WorkerName string
}

//go:generate counterfeiter . WorkerTaskCacheFactory

type WorkerTaskCacheFactory interface {
	Find(jobID int, stepName string, path string, workerName string) (*UsedWorkerTaskCache, bool, error)
	FindOrCreate(jobID int, stepName string, path string, workerName string) (*UsedWorkerTaskCache, error)
}

type workerTaskCacheFactory struct {
	conn Conn
}

func NewWorkerTaskCacheFactory(conn Conn) WorkerTaskCacheFactory {
	return &workerTaskCacheFactory{
		conn: conn,
	}
}

func (f *workerTaskCacheFactory) Find(jobID int, stepName string, path string, workerName string) (*UsedWorkerTaskCache, bool, error) {
	var id int
	err := psql.Select("id").
		From("worker_task_caches").
		Where(sq.Eq{
			"job_id":      jobID,
			"step_name":   stepName,
			"worker_name": workerName,
			"path":        path,
		}).
		RunWith(f.conn).
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
		WorkerName: workerName,
	}, true, nil
}

func (f *workerTaskCacheFactory) FindOrCreate(jobID int, stepName string, path string, workerName string) (*UsedWorkerTaskCache, error) {
	workerTaskCache := WorkerTaskCache{
		JobID:      jobID,
		StepName:   stepName,
		WorkerName: workerName,
		Path:       path,
	}

	tx, err := f.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	usedWorkerTaskCache, err := workerTaskCache.FindOrCreate(tx)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return usedWorkerTaskCache, nil
}

type WorkerTaskCache struct {
	JobID      int
	StepName   string
	WorkerName string
	Path       string
}

func (wtc WorkerTaskCache) FindOrCreate(
	tx Tx,
) (*UsedWorkerTaskCache, error) {
	var id int
	err := psql.Select("id").
		From("worker_task_caches").
		Where(sq.Eq{
			"job_id":      wtc.JobID,
			"step_name":   wtc.StepName,
			"worker_name": wtc.WorkerName,
			"path":        wtc.Path,
		}).
		RunWith(tx).
		QueryRow().
		Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			err = psql.Insert("worker_task_caches").
				Columns(
					"job_id",
					"step_name",
					"worker_name",
					"path",
				).
				Values(
					wtc.JobID,
					wtc.StepName,
					wtc.WorkerName,
					wtc.Path,
				).
				Suffix(`
					ON CONFLICT (job_id, step_name, worker_name, path) DO UPDATE SET
						path = ?
					RETURNING id
				`, wtc.Path).
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

func removeUnusedWorkerTaskCaches(tx Tx, pipelineID int, jobConfigs []atc.JobConfig) error {
	steps := make(map[string][]string)
	for _, jobConfig := range jobConfigs {
		for _, jobConfigPlan := range jobConfig.Plan {
			if jobConfigPlan.Task != "" {
				steps[jobConfig.Name] = append(steps[jobConfig.Name], jobConfigPlan.Task)
			}
		}
	}

	query := sq.Or{}
	for jobName, stepNames := range steps {
		query = append(query, sq.And{sq.Eq{"j.name": jobName}, sq.NotEq{"wtc.step_name": stepNames}})
	}

	_, err := psql.Delete("worker_task_caches wtc USING jobs j").
		Where(sq.Or{
			query,
			sq.Eq{
				"j.pipeline_id": pipelineID,
				"j.active":      false,
			},
		}).
		Where(sq.Expr("j.id = wtc.job_id")).
		RunWith(tx).
		Exec()

	return err
}
