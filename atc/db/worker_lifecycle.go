package db

import (
	"database/sql"

	sq "github.com/Masterminds/squirrel"
)

//go:generate counterfeiter . WorkerLifecycle

type WorkerLifecycle interface {
	DeleteUnresponsiveEphemeralWorkers() ([]string, error)
	StallUnresponsiveWorkers() ([]string, error)
	LandFinishedLandingWorkers() ([]string, error)
	DeleteFinishedRetiringWorkers() ([]string, error)
}

type workerLifecycle struct {
	conn Conn
}

func NewWorkerLifecycle(conn Conn) WorkerLifecycle {
	return &workerLifecycle{
		conn: conn,
	}
}

func (lifecycle *workerLifecycle) DeleteUnresponsiveEphemeralWorkers() ([]string, error) {
	query, args, err := psql.Delete("workers").
		Where(sq.Eq{"ephemeral": true}).
		Where(sq.Expr("expires < NOW()")).
		Suffix("RETURNING name").
		ToSql()

	if err != nil {
		return []string{}, err
	}

	rows, err := lifecycle.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}

	return workersAffected(rows)
}

func (lifecycle *workerLifecycle) StallUnresponsiveWorkers() ([]string, error) {
	query, args, err := psql.Update("workers").
		SetMap(map[string]interface{}{
			"state":            string(WorkerStateStalled),
			"addr":             nil,
			"baggageclaim_url": nil,
			"expires":          nil,
		}).
		Where(sq.Eq{"state": string(WorkerStateRunning)}).
		Where(sq.Expr("expires < NOW()")).
		Suffix("RETURNING name").
		ToSql()
	if err != nil {
		return []string{}, err
	}

	rows, err := lifecycle.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}

	return workersAffected(rows)
}

func (lifecycle *workerLifecycle) DeleteFinishedRetiringWorkers() ([]string, error) {
	// Squirrel does not have default support for subqueries in where clauses.
	// We hacked together a way to do it
	//
	// First we generate the subquery's SQL and args using
	// sq.Select instead of psql.Select so that we get
	// unordered placeholders instead of psql's ordered placeholders
	subQ, subQArgs, err := sq.Select("w.name").
		Distinct().
		From("builds b").
		Join("containers c ON b.id = c.build_id").
		Join("workers w ON w.name = c.worker_name").
		LeftJoin("jobs j ON j.id = b.job_id").
		Where(sq.Or{
			sq.Eq{
				"b.status": string(BuildStatusStarted),
			},
			sq.Eq{
				"b.status": string(BuildStatusPending),
			},
		}).
		Where(sq.Or{
			sq.Eq{
				"j.interruptible": false,
			},
			sq.Eq{
				"b.job_id": nil,
			},
		}).ToSql()

	if err != nil {
		return []string{}, err
	}

	// Then we inject the subquery sql directly into
	// the where clause, and "add" the args from the
	// first query to the second query's args
	//
	// We use sq.Delete instead of psql.Delete for the same reason
	// but then change the placeholders using .PlaceholderFormat(sq.Dollar)
	// to go back to postgres's format
	query, args, err := sq.Delete("workers").
		Where(sq.Eq{
			"state": string(WorkerStateRetiring),
		}).
		Where("name NOT IN ("+subQ+")", subQArgs...).
		PlaceholderFormat(sq.Dollar).
		Suffix("RETURNING name").
		ToSql()

	if err != nil {
		return []string{}, err
	}

	rows, err := lifecycle.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}

	return workersAffected(rows)
}

func (lifecycle *workerLifecycle) LandFinishedLandingWorkers() ([]string, error) {
	subQ, subQArgs, err := sq.Select("w.name").
		Distinct().
		From("builds b").
		Join("containers c ON b.id = c.build_id").
		Join("workers w ON w.name = c.worker_name").
		LeftJoin("jobs j ON j.id = b.job_id").
		Where(sq.Or{
			sq.Eq{
				"b.status": string(BuildStatusStarted),
			},
			sq.Eq{
				"b.status": string(BuildStatusPending),
			},
		}).
		Where(sq.Or{
			sq.Eq{
				"j.interruptible": false,
			},
			sq.Eq{
				"b.job_id": nil,
			},
		}).ToSql()

	if err != nil {
		return nil, err
	}

	query, args, err := sq.Update("workers").
		Set("state", string(WorkerStateLanded)).
		Set("addr", nil).
		Set("baggageclaim_url", nil).
		Where(sq.Eq{
			"state": string(WorkerStateLanding),
		}).
		Where("name NOT IN ("+subQ+")", subQArgs...).
		PlaceholderFormat(sq.Dollar).
		Suffix("RETURNING name").
		ToSql()

	if err != nil {
		return []string{}, err
	}

	rows, err := lifecycle.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}

	return workersAffected(rows)
}

func workersAffected(rows *sql.Rows) ([]string, error) {
	var (
		err         error
		workerNames []string
	)

	defer Close(rows)

	for rows.Next() {
		var name string

		err = rows.Scan(&name)
		if err != nil {
			return nil, err
		}

		workerNames = append(workerNames, name)
	}

	return workerNames, err
}
