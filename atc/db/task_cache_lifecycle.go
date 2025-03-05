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
	rows, err := psql.Delete("task_caches tc").
		Where(sq.Expr(`tc.id IN (
            SELECT tc.id 
            FROM task_caches tc
            JOIN jobs j ON j.id = tc.job_id
            JOIN pipelines p ON p.id = j.pipeline_id
            WHERE p.archived OR
                (p.paused AND j.next_build_id IS NULL) OR
                (j.paused AND j.next_build_id IS NULL)
        )`)).
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
