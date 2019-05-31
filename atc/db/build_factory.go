package db

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/v5/atc/db/lock"
)

//go:generate counterfeiter . BuildFactory

type BuildFactory interface {
	Build(int) (Build, bool, error)
	VisibleBuilds([]string, Page) ([]Build, Pagination, error)
	VisibleBuildsWithTime([]string, Page) ([]Build, Pagination, error)
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

func (f *buildFactory) VisibleBuildsWithTime(teamNames []string, page Page) ([]Build, Pagination, error) {
	newBuildsQuery := buildsQuery.
		Where(sq.Or{
			sq.Eq{"p.public": true},
			sq.Eq{"t.name": teamNames},
		})
	return getBuildsWithDates(newBuildsQuery, minMaxIdQuery, page, f.conn, f.lockFactory)
}

func (f *buildFactory) VisibleBuilds(teamNames []string, page Page) ([]Build, Pagination, error) {
	newBuildsQuery := buildsQuery.
		Where(sq.Or{
			sq.Eq{"p.public": true},
			sq.Eq{"t.name": teamNames},
		})

	return getBuildsWithPagination(newBuildsQuery, minMaxIdQuery,
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

func getBuildsWithDates(buildsQuery, minMaxIdQuery sq.SelectBuilder, page Page, conn Conn, lockFactory lock.LockFactory) ([]Build, Pagination, error) {
	var newPage = Page{Limit: page.Limit}

	if page.Since != 0 {
		sinceRow, err := buildsQuery.
			Where(sq.Expr("b.start_time >= to_timestamp(" + strconv.Itoa(page.Since) + ")")).
			OrderBy("b.id ASC").
			Limit(1).
			RunWith(conn).
			Query()

		if err != nil {
			// The user has no builds since that given time
			if err == sql.ErrNoRows {
				return []Build{}, Pagination{}, nil
			}

			return nil, Pagination{}, err
		}

		defer sinceRow.Close()

		for sinceRow.Next() {
			build := &build{conn: conn, lockFactory: lockFactory}
			err = scanBuild(build, sinceRow, conn.EncryptionStrategy())
			if err != nil {
				return nil, Pagination{}, err
			}

			// Subtracting one in order to make the range inclusive
			// of the current build.ID() since the getBuildsWithPagination
			// is exclusive.
			//
			// Setting `Until` instead of `Since` to adapt to the point
			// of view of pagination.
			newPage.Until = build.ID() - 1
		}
	}

	if page.Until != 0 {
		untilRow, err := buildsQuery.
			Where(sq.Expr("b.start_time <= to_timestamp(" + strconv.Itoa(page.Until) + ")")).
			OrderBy("b.id DESC").
			Limit(1).
			RunWith(conn).
			Query()
		if err != nil {
			// The user has no builds since that given time
			if err == sql.ErrNoRows {
				return []Build{}, Pagination{}, nil
			}
		}

		defer untilRow.Close()
		for untilRow.Next() {
			build := &build{conn: conn, lockFactory: lockFactory}
			err = scanBuild(build, untilRow, conn.EncryptionStrategy())
			if err != nil {
				return nil, Pagination{}, err
			}

			// Adding one in order to make the range inclusive
			// of the current build.ID() Since the getBuildsWithPagination
			// is exclusive.
			//
			// Setting `Since` instead of `Until` to adapt to the point
			// of view of pagination.
			newPage.Since = build.ID() + 1
		}
	}

	return getBuildsWithPagination(buildsQuery, minMaxIdQuery, newPage, conn, lockFactory)
}

func getBuildsWithPagination(buildsQuery, minMaxIdQuery sq.SelectBuilder, page Page, conn Conn, lockFactory lock.LockFactory) ([]Build, Pagination, error) {
	var (
		rows    *sql.Rows
		err     error
		reverse bool
	)

	buildsQuery = buildsQuery.Limit(uint64(page.Limit))

	if page.Since == 0 && page.Until == 0 { // none
		buildsQuery = buildsQuery.
			OrderBy("b.id DESC")
	} else if page.Until != 0 && page.Since == 0 { // only until
		buildsQuery = buildsQuery.
			Where(sq.Gt{"b.id": uint64(page.Until)}).
			OrderBy("b.id ASC")
		reverse = true
	} else if page.Since != 0 && page.Until == 0 { // only since
		buildsQuery = buildsQuery.
			Where(sq.Lt{"b.id": page.Since}).
			OrderBy("b.id DESC")
	} else if page.Until != 0 && page.Since != 0 { // both
		if page.Until > page.Since {
			return nil, Pagination{}, fmt.Errorf("Invalid range boundaries")
		}

		buildsQuery = buildsQuery.Where(
			sq.And{
				sq.Gt{"b.id": uint64(page.Until)},
				sq.Lt{"b.id": uint64(page.Since)},
			}).
			OrderBy("b.id ASC")
	}

	rows, err = buildsQuery.RunWith(conn).Query()
	if err != nil {
		return nil, Pagination{}, err
	}

	defer Close(rows)

	builds := make([]Build, 0)
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

	var minID, maxID int
	err = minMaxIdQuery.
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
