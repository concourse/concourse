package db

import (
	"database/sql"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
)

type WorkerTaskCache struct {
	WorkerName string
	TaskCache  UsedTaskCache
}

type UsedWorkerTaskCache struct {
	ID         int
	WorkerName string
}

func (wtc WorkerTaskCache) findOrCreate(
	tx Tx,
) (*UsedWorkerTaskCache, error) {
	uwtc, found, err := wtc.find(tx)
	if err != nil {
		return nil, err
	}

	if !found {
		var id int
		err = psql.Insert("worker_task_caches").
			Columns(
				"worker_name",
				"task_cache_id",
			).
			Values(wtc.WorkerName, wtc.TaskCache.ID()).
			Suffix(`
					ON CONFLICT (worker_name, task_cache_id) DO UPDATE SET
						task_cache_id = EXCLUDED.task_cache_id
					RETURNING id
				`).
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

	return uwtc, err
}

func (workerTaskCache WorkerTaskCache) find(runner sq.Runner) (*UsedWorkerTaskCache, bool, error) {
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

func removeUnusedWorkerTaskCaches(tx Tx, pipelineID int, jobConfigs []atc.JobConfig) error {
	steps := make(map[string][]string)
	for _, jobConfig := range jobConfigs {
		_ = jobConfig.StepConfig().Visit(atc.StepRecursor{
			OnTask: func(step *atc.TaskStep) error {
				steps[jobConfig.Name] = append(steps[jobConfig.Name], step.Name)
				return nil
			},
		})
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
