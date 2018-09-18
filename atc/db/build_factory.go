package db

import (
	"database/sql"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc/db/lock"
)

//go:generate counterfeiter . BuildFactory

type BuildFactory interface {
	Build(int) (Build, bool, error)
	VisibleBuilds([]string, Page) ([]Build, Pagination, error)
	PublicBuilds(Page) ([]Build, Pagination, error)
	GetAllStartedBuilds() ([]Build, error)
	GetDrainableBuilds() ([]Build, error)
	// TODO: move to BuildLifecycle, new interface (see WorkerLifecycle)
	MarkNonInterceptibleBuilds() error
}

type buildFactory struct {
	conn              Conn
	lockFactory       lock.LockFactory
	oneOffGracePeriod time.Duration
}

func NewBuildFactory(conn Conn, lockFactory lock.LockFactory, oneOffGracePeriod time.Duration) BuildFactory {
	return &buildFactory{
		conn:              conn,
		lockFactory:       lockFactory,
		oneOffGracePeriod: oneOffGracePeriod,
	}
}

func (f *buildFactory) Build(buildID int) (Build, bool, error) {
	build := &build{
		conn:        f.conn,
		lockFactory: f.lockFactory,
	}

	row := buildsQuery.
		Where(sq.Eq{"b.id": buildID}).
		RunWith(f.conn).
		QueryRow()

	err := scanBuild(build, row, f.conn.EncryptionStrategy())
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	return build, true, nil
}

func (f *buildFactory) VisibleBuilds(teamNames []string, page Page) ([]Build, Pagination, error) {
	newBuildsQuery := buildsQuery.
		Where(sq.Or{
			sq.Eq{"p.public": true},
			sq.Eq{"t.name": teamNames},
		})

	return getBuildsWithPagination(newBuildsQuery, page, f.conn, f.lockFactory)
}

func (f *buildFactory) PublicBuilds(page Page) ([]Build, Pagination, error) {
	return getBuildsWithPagination(buildsQuery.Where(sq.Eq{"p.public": true}), page, f.conn, f.lockFactory)
}

func (f *buildFactory) MarkNonInterceptibleBuilds() error {
	_, err := psql.Update("builds b").
		Set("interceptible", false).
		Where(sq.Eq{
			"completed":     true,
			"interceptible": true,
		}).
		Where(sq.Or{
			sq.NotEq{"job_id": nil},
			sq.Expr(fmt.Sprintf("now() - end_time > '%d seconds'::interval", int(f.oneOffGracePeriod.Seconds()))),
		}).
		Where(sq.Or{
			sq.Expr("NOT EXISTS (SELECT 1 FROM jobs j WHERE j.latest_completed_build_id = b.id)"),
			sq.Eq{"status": string(BuildStatusSucceeded)},
		}).
		RunWith(f.conn).
		Exec()
	return err
}

func (f *buildFactory) GetDrainableBuilds() ([]Build, error) {
	query := buildsQuery.Where(sq.Eq{
		"b.completed": true,
		"b.drained":   false,
	})

	return getBuilds(query, f.conn, f.lockFactory)
}

func (f *buildFactory) GetAllStartedBuilds() ([]Build, error) {
	query := buildsQuery.Where(sq.Eq{
		"b.status": BuildStatusStarted,
	})

	return getBuilds(query, f.conn, f.lockFactory)
}

func getBuilds(buildsQuery sq.SelectBuilder, conn Conn, lockFactory lock.LockFactory) ([]Build, error) {
	rows, err := buildsQuery.RunWith(conn).Query()
	if err != nil {
		return nil, err
	}

	defer Close(rows)

	bs := []Build{}

	for rows.Next() {
		b := &build{conn: conn, lockFactory: lockFactory}
		err := scanBuild(b, rows, conn.EncryptionStrategy())
		if err != nil {
			return nil, err
		}

		bs = append(bs, b)
	}

	return bs, nil
}

func getBuildsWithPagination(buildsQuery sq.SelectBuilder, page Page, conn Conn, lockFactory lock.LockFactory) ([]Build, Pagination, error) {
	var rows *sql.Rows
	var err error

	var reverse bool
	if page.Since == 0 && page.Until == 0 {
		buildsQuery = buildsQuery.OrderBy("b.id DESC").Limit(uint64(page.Limit))
	} else if page.Until != 0 {
		buildsQuery = buildsQuery.Where(sq.Gt{"b.id": uint64(page.Until)}).OrderBy("b.id ASC").Limit(uint64(page.Limit))
		reverse = true
	} else {
		buildsQuery = buildsQuery.Where(sq.Lt{"b.id": page.Since}).OrderBy("b.id DESC").Limit(uint64(page.Limit))
	}

	rows, err = buildsQuery.RunWith(conn).Query()
	if err != nil {
		return nil, Pagination{}, err
	}

	defer Close(rows)

	builds := []Build{}

	for rows.Next() {
		build := &build{conn: conn, lockFactory: lockFactory}
		err = scanBuild(build, rows, conn.EncryptionStrategy())
		if err != nil {
			return nil, Pagination{}, err
		}

		builds = append(builds, build)
	}

	if reverse {
		for i, j := 0, len(builds)-1; i < j; i, j = i+1, j-1 {
			builds[i], builds[j] = builds[j], builds[i]
		}
	}

	if len(builds) == 0 {
		return builds, Pagination{}, nil
	}

	var minID int
	var maxID int
	err = psql.Select("COALESCE(MAX(id), 0)", "COALESCE(MIN(id), 0)").
		From("builds").
		RunWith(conn).
		QueryRow().
		Scan(&maxID, &minID)
	if err != nil {
		return nil, Pagination{}, err
	}

	first := builds[0]
	last := builds[len(builds)-1]

	var pagination Pagination

	if first.ID() < maxID {
		pagination.Previous = &Page{
			Until: first.ID(),
			Limit: page.Limit,
		}
	}

	if last.ID() > minID {
		pagination.Next = &Page{
			Since: last.ID(),
			Limit: page.Limit,
		}
	}

	return builds, pagination, nil
}
