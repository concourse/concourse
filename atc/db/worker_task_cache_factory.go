package db

import (
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
)

//go:generate counterfeiter . WorkerTaskCacheFactory

type WorkerTaskCacheFactory interface {
	FindOrCreate(WorkerTaskCache) (*UsedWorkerTaskCache, error)
	Find(WorkerTaskCache) (*UsedWorkerTaskCache, bool, error)
}

type workerTaskCacheFactory struct {
	conn Conn
}

func NewWorkerTaskCacheFactory(conn Conn) WorkerTaskCacheFactory {
	return &workerTaskCacheFactory{
		conn: conn,
	}
}

func (f *workerTaskCacheFactory) FindOrCreate(workerTaskCache WorkerTaskCache) (*UsedWorkerTaskCache, error) {
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

func (f *workerTaskCacheFactory) Find(workerTaskCache WorkerTaskCache) (*UsedWorkerTaskCache, bool, error) {
	tx, err := f.conn.Begin()
	if err != nil {
		return nil, false, err
	}

	defer Rollback(tx)

	usedWorkerTaskCache, found, err := workerTaskCache.Find(tx)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	err = tx.Commit()
	if err != nil {
		return nil, false, err
	}

	return usedWorkerTaskCache, true, nil
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
		query = append(query, sq.And{sq.Eq{"j.name": jobName}, sq.NotEq{"tc.step_name": stepNames}})
	}

	_, err := psql.Delete("task_caches tc USING jobs j").
		Where(
			sq.Or{
				query,
				sq.Eq{
					"j.active": false,
				},
			}).
		Where(sq.Expr("j.id = tc.job_id")).
		Where(sq.Eq{"j.pipeline_id": pipelineID}).
		RunWith(tx).
		Exec()

	return err
}
