package db

import (
	sq "github.com/Masterminds/squirrel"
)

//counterfeiter:generate . TaskCacheLifecycle
type TaskCacheLifecycle interface {
	CleanUpInvalidTaskCaches() ([]int, error)
}

type taskCacheLifecycle struct {
	conn DbConn
}

func NewTaskCacheLifecycle(conn DbConn) TaskCacheLifecycle {
	return &taskCacheLifecycle{
		conn: conn,
	}
}

func (f *taskCacheLifecycle) CleanUpInvalidTaskCaches() ([]int, error) {
	inactiveTaskCaches, _, err := psql.Select("tc.id").
		From("task_caches tc").
		Join("jobs j ON j.id = tc.job_id").
		Join("pipelines p ON p.id = j.pipeline_id").
		Where(sq.Or{
			sq.Expr("p.archived"),
			sq.And{
				sq.Expr("p.paused"),
				sq.Expr("j.next_build_id IS NULL"),
			},
			sq.And{
				sq.Expr("j.paused"),
				sq.Expr("j.next_build_id IS NULL"),
			},
		}).
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := psql.Delete("task_caches").
		Where("id IN (" + inactiveTaskCaches + ")").
		Suffix("RETURNING id").
		RunWith(f.conn).
		Query()
	if err != nil {
		return nil, err
	}

	defer Close(rows)

	var deletedCacheIDs []int
	for rows.Next() {
		var cacheID int
		err = rows.Scan(&cacheID)
		if err != nil {
			return nil, err
		}

		deletedCacheIDs = append(deletedCacheIDs, cacheID)
	}

	return deletedCacheIDs, nil
}
