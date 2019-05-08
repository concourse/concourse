package db

import (
	"database/sql"

	sq "github.com/Masterminds/squirrel"
)

//go:generate counterfeiter . TaskCacheFactory

type TaskCacheFactory interface {
	Find(jobID int, stepName string, path string) (UsedTaskCache, bool, error)
	FindOrCreate(jobID int, stepName string, path string) (UsedTaskCache, error)
}

type taskCacheFactory struct {
	conn Conn
}

func NewTaskCacheFactory(conn Conn) TaskCacheFactory {
	return &taskCacheFactory{
		conn: conn,
	}
}

func (f *taskCacheFactory) Find(jobID int, stepName string, path string) (UsedTaskCache, bool, error) {
	taskCache, found, err := f.find(jobID, stepName, path)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	return taskCache, true, nil
}

func (f *taskCacheFactory) FindOrCreate(jobID int, stepName string, path string) (UsedTaskCache, error) {
	utc, found, err := f.find(jobID, stepName, path)
	if err != nil {
		return nil, err
	}

	if !found {
		var id int
		err = psql.Insert("task_caches").
			Columns(
				"job_id",
				"step_name",
				"path",
			).
			Values(
				jobID,
				stepName,
				path,
			).
			Suffix(`
					ON CONFLICT (job_id, step_name, path) DO UPDATE SET
						job_id = ?
					RETURNING id
				`, jobID).
			RunWith(f.conn).
			QueryRow().
			Scan(&id)
		if err != nil {
			return nil, err
		}

		return &usedTaskCache{
			id:       id,
			jobID:    jobID,
			stepName: stepName,
			path:     path,
		}, nil
	}

	return utc, nil
}

func (f *taskCacheFactory) find(jobID int, stepName string, path string) (UsedTaskCache, bool, error) {
	var id int
	err := psql.Select("id").
		From("task_caches").
		Where(sq.Eq{
			"job_id":    jobID,
			"step_name": stepName,
			"path":      path,
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

	return &usedTaskCache{
		id:       id,
		jobID:    jobID,
		stepName: stepName,
		path:     path,
	}, true, nil

}
