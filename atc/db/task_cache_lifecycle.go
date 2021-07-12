package db

import (
	sq "github.com/Masterminds/squirrel"
)

//counterfeiter:generate . TaskCacheLifecycle
type TaskCacheLifecycle interface {
	CleanUpInvalidTaskCaches() ([]int, error)
}

type taskCacheLifecycle struct {
	conn Conn
}

func NewTaskCacheLifecycle(conn Conn) TaskCacheLifecycle {
	return &taskCacheLifecycle{
		conn: conn,
	}
}

func (f *taskCacheLifecycle) CleanUpInvalidTaskCaches() ([]int, error) {
	inactiveTaskCaches, _, err := psql.Select("tc.id").
		From("task_caches tc").
		Join("jobs j ON j.id = tc.job_id").
		Join("pipelines p ON p.id = j.pipeline_id").
		Where(sq.Expr("p.archived")).
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
