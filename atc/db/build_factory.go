package db

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc/db/lock"
)

//go:generate counterfeiter . BuildFactory

type BuildFactory interface {
	Build(int) (Build, bool, error)
	VisibleBuilds([]string, Page) ([]Build, Pagination, error)
	AllBuilds(Page) ([]Build, Pagination, error)
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
	failedGracePeriod time.Duration
}

func NewBuildFactory(conn Conn, lockFactory lock.LockFactory, oneOffGracePeriod time.Duration, failedGracePeriod time.Duration) BuildFactory {
	return &buildFactory{
		conn:              conn,
		lockFactory:       lockFactory,
		oneOffGracePeriod: oneOffGracePeriod,
		failedGracePeriod: failedGracePeriod,
	}
}

func (f *buildFactory) Build(buildID int) (Build, bool, error) {
	build := newEmptyBuild(f.conn, f.lockFactory)
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

	if page.UseDate {
		return getBuildsWithDates(newBuildsQuery, minMaxIdQuery, page, f.conn,
			f.lockFactory)
	}
	return getBuildsWithPagination(newBuildsQuery, minMaxIdQuery, page, f.conn,
		f.lockFactory)
}

func (f *buildFactory) AllBuilds(page Page) ([]Build, Pagination, error) {
	if page.UseDate {
		return getBuildsWithDates(buildsQuery, minMaxIdQuery, page, f.conn,
			f.lockFactory)
	}
	return getBuildsWithPagination(buildsQuery, minMaxIdQuery,
		page, f.conn, f.lockFactory)
}

func (f *buildFactory) PublicBuilds(page Page) ([]Build, Pagination, error) {
	return getBuildsWithPagination(
		buildsQuery.Where(sq.Eq{"p.public": true}), minMaxIdQuery,
		page, f.conn, f.lockFactory)
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
		Where(f.constructBuildFilter()).
		RunWith(f.conn).
		Exec()
	return err
}

func (f *buildFactory) constructBuildFilter() sq.Or {
	buildFilter := sq.Or{
		sq.Expr("NOT EXISTS (SELECT 1 FROM jobs j WHERE j.latest_completed_build_id = b.id)"),
		sq.Eq{"status": string(BuildStatusSucceeded)},
	}
	if f.failedGracePeriod > 0 { // if zero, grace period is disabled
		buildFilter = append(buildFilter,
			sq.Expr(fmt.Sprintf("now() - end_time > '%d seconds'::interval", int(f.failedGracePeriod.Seconds()))))
	}
	return buildFilter
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
		b := newEmptyBuild(conn, lockFactory)
		err := scanBuild(b, rows, conn.EncryptionStrategy())
		if err != nil {
			return nil, err
		}

		bs = append(bs, b)
	}

	return bs, nil
}

func getBuildsWithDates(buildsQuery, minMaxIdQuery sq.SelectBuilder, page Page, conn Conn, lockFactory lock.LockFactory) ([]Build, Pagination, error) {
	var newPage = Page{Limit: page.Limit}

	tx, err := conn.Begin()
	if err != nil {
		return nil, Pagination{}, err
	}

	defer Rollback(tx)

	if page.From != 0 {
		fromRow, err := buildsQuery.
			Where(sq.Expr("b.start_time >= to_timestamp(" + strconv.Itoa(page.From) + ")")).
			OrderBy("COALESCE(b.rerun_of, b.id) ASC, b.id ASC").
			Limit(1).
			RunWith(tx).
			Query()

		if err != nil {
			// The user has no builds since that given time
			if err == sql.ErrNoRows {
				return []Build{}, Pagination{}, nil
			}

			return nil, Pagination{}, err
		}

		defer fromRow.Close()

		found := false
		for fromRow.Next() {
			found = true
			build := newEmptyBuild(conn, lockFactory)
			err = scanBuild(build, fromRow, conn.EncryptionStrategy())
			if err != nil {
				return nil, Pagination{}, err
			}

			newPage.From = build.ID()
		}
		if !found {
			return []Build{}, Pagination{}, nil
		}
	}

	if page.To != 0 {
		untilRow, err := buildsQuery.
			Where(sq.Expr("b.start_time <= to_timestamp(" + strconv.Itoa(page.To) + ")")).
			OrderBy("COALESCE(b.rerun_of, b.id) DESC, b.id DESC").
			Limit(1).
			RunWith(tx).
			Query()
		if err != nil {
			// The user has no builds since that given time
			if err == sql.ErrNoRows {
				return []Build{}, Pagination{}, nil
			}
		}

		defer untilRow.Close()

		found := false
		for untilRow.Next() {
			found = true
			build := newEmptyBuild(conn, lockFactory)
			err = scanBuild(build, untilRow, conn.EncryptionStrategy())
			if err != nil {
				return nil, Pagination{}, err
			}

			newPage.To = build.ID()
		}
		if !found {
			return []Build{}, Pagination{}, nil
		}
	}

	err = tx.Commit()
	if err != nil {
		return nil, Pagination{}, err
	}

	return getBuildsWithPagination(buildsQuery, minMaxIdQuery, newPage, conn, lockFactory)
}

func getBuildsWithPagination(buildsQuery, minMaxIdQuery sq.SelectBuilder, page Page, conn Conn, lockFactory lock.LockFactory) ([]Build, Pagination, error) {
	var (
		rows    *sql.Rows
		err     error
		reverse bool
	)

	origBuildsQuery := buildsQuery

	tx, err := conn.Begin()
	if err != nil {
		return nil, Pagination{}, err
	}

	defer Rollback(tx)

	buildsQuery = buildsQuery.Limit(uint64(page.Limit))

	if page.From == 0 && page.To == 0 { // none
		buildsQuery = buildsQuery.
			OrderBy("COALESCE(b.rerun_of, b.id) DESC, b.id DESC")
	} else if page.From != 0 && page.To == 0 { // only from
		buildsQuery = buildsQuery.
			Where(sq.GtOrEq{"b.id": uint64(page.From)}).
			OrderBy("COALESCE(b.rerun_of, b.id) ASC, b.id ASC")
		reverse = true
	} else if page.From == 0 && page.To != 0 { // only to
		buildsQuery = buildsQuery.
			Where(sq.LtOrEq{"b.id": page.To}).
			OrderBy("COALESCE(b.rerun_of, b.id) DESC, b.id DESC")
	} else if page.From != 0 && page.To != 0 { // both
		if page.From > page.To {
			return nil, Pagination{}, fmt.Errorf("Invalid range boundaries")
		}

		buildsQuery = buildsQuery.Where(
			sq.And{
				sq.GtOrEq{"b.id": uint64(page.From)},
				sq.LtOrEq{"b.id": uint64(page.To)},
			}).
			OrderBy("COALESCE(b.rerun_of, b.id) ASC, b.id ASC")
	}

	rows, err = buildsQuery.RunWith(tx).Query()
	if err != nil {
		return nil, Pagination{}, err
	}

	defer Close(rows)

	builds := make([]Build, 0)
	for rows.Next() {
		build := newEmptyBuild(conn, lockFactory)
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

	newestBuild := builds[0]
	oldestBuild := builds[len(builds)-1]

	var pagination Pagination

	row := origBuildsQuery.
		Where(sq.Lt{"b.id": oldestBuild.ID()}).
		OrderBy("COALESCE(b.rerun_of, b.id) DESC, b.id DESC").
		Limit(1).
		RunWith(tx).
		QueryRow()

	build := newEmptyBuild(conn, lockFactory)
	err = scanBuild(build, row, conn.EncryptionStrategy())
	if err != nil && err != sql.ErrNoRows {
		return builds, Pagination{}, err
	} else if err == nil {
		pagination.Older = &Page{
			To:    build.id,
			Limit: page.Limit,
		}
	}

	row = origBuildsQuery.
		Where(sq.Gt{"b.id": newestBuild.ID()}).
		OrderBy("COALESCE(b.rerun_of, b.id) ASC, b.id ASC").
		Limit(1).
		RunWith(tx).
		QueryRow()

	build = newEmptyBuild(conn, lockFactory)
	err = scanBuild(build, row, conn.EncryptionStrategy())
	if err != nil && err != sql.ErrNoRows {
		return builds, Pagination{}, err
	} else if err == nil {
		pagination.Newer = &Page{
			From:  build.id,
			Limit: page.Limit,
		}
	}

	err = tx.Commit()
	if err != nil {
		return nil, Pagination{}, err
	}

	return builds, pagination, nil
}
