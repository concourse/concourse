package db

import (
	"database/sql"
	"time"

	sq "github.com/Masterminds/squirrel"
)

type usedTaskCache struct {
	id       int
	jobID    int
	stepName string
	path     string
	ttl      time.Duration
}

type UsedTaskCache interface {
	ID() int

	JobID() int
	StepName() string
	Path() string
	TTL() time.Duration
}

func (tc *usedTaskCache) ID() int            { return tc.id }
func (tc *usedTaskCache) JobID() int         { return tc.jobID }
func (tc *usedTaskCache) StepName() string   { return tc.stepName }
func (tc *usedTaskCache) Path() string       { return tc.path }
func (tc *usedTaskCache) TTL() time.Duration { return tc.ttl }

func (f usedTaskCache) findOrCreate(tx Tx) (UsedTaskCache, error) {
	utc, found, err := f.find(tx)
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
				"last_used",
				"ttl",
			).
			Values(
				f.jobID,
				f.stepName,
				f.path,
				sq.Expr("now()"),
				int64(f.ttl.Seconds()),
			).
			Suffix(`
					ON CONFLICT (job_id, step_name, path) DO UPDATE SET
						last_used = now(),
						ttl = ?
					RETURNING id
				`, int64(f.ttl.Seconds())).
			RunWith(tx).
			QueryRow().
			Scan(&id)
		if err != nil {
			return nil, err
		}

		return &usedTaskCache{
			id:       id,
			jobID:    f.jobID,
			stepName: f.stepName,
			path:     f.path,
			ttl:      f.ttl,
		}, nil
	}

	return utc, nil
}

func (f usedTaskCache) find(runner sq.Runner) (UsedTaskCache, bool, error) {
	var (
		id         int
		ttlSeconds int64
	)
	err := psql.Select("id", "ttl").
		From("task_caches").
		Where(sq.Eq{
			"job_id":    f.jobID,
			"step_name": f.stepName,
			"path":      f.path,
		}).
		RunWith(runner).
		QueryRow().
		Scan(&id, &ttlSeconds)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}

		return nil, false, err
	}

	return &usedTaskCache{
		id:       id,
		jobID:    f.jobID,
		stepName: f.stepName,
		path:     f.path,
		ttl:      time.Duration(ttlSeconds) * time.Second,
	}, true, nil

}
