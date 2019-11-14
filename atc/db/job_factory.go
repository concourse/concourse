package db

import (
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc/db/lock"
)

//go:generate counterfeiter . JobFactory

type JobFactory interface {
	VisibleJobs([]string) (Dashboard, error)
	AllActiveJobs() (Dashboard, error)
}

type jobFactory struct {
	conn        Conn
	lockFactory lock.LockFactory
}

func NewJobFactory(conn Conn, lockFactory lock.LockFactory) JobFactory {
	return &jobFactory{
		conn:        conn,
		lockFactory: lockFactory,
	}
}

func (j *jobFactory) VisibleJobs(teamNames []string) (Dashboard, error) {
	tx, err := j.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	rows, err := jobsQuery.
		Where(sq.Eq{
			"j.active": true,
		}).
		Where(sq.Or{
			sq.Eq{"t.name": teamNames},
			sq.Eq{"p.public": true},
		}).
		OrderBy("j.id ASC").
		RunWith(tx).
		Query()
	if err != nil {
		return nil, err
	}

	jobs, err := scanJobs(j.conn, j.lockFactory, rows)
	if err != nil {
		return nil, err
	}

	dashboard, err := j.buildDashboard(tx, jobs)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return dashboard, nil
}

func (j *jobFactory) AllActiveJobs() (Dashboard, error) {
	tx, err := j.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	rows, err := jobsQuery.
		Where(sq.Eq{
			"j.active": true,
		}).
		OrderBy("j.id ASC").
		RunWith(tx).
		Query()
	if err != nil {
		return nil, err
	}

	jobs, err := scanJobs(j.conn, j.lockFactory, rows)
	if err != nil {
		return nil, err
	}

	dashboard, err := j.buildDashboard(tx, jobs)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return dashboard, nil
}

func (j *jobFactory) buildDashboard(tx Tx, jobs Jobs) (Dashboard, error) {
	var jobIDs []int
	for _, job := range jobs {
		jobIDs = append(jobIDs, job.ID())
	}

	nextBuilds, err := j.getBuildsFrom(tx, "next_build_id", jobIDs)
	if err != nil {
		return nil, err
	}

	finishedBuilds, err := j.getBuildsFrom(tx, "latest_completed_build_id", jobIDs)
	if err != nil {
		return nil, err
	}

	transitionBuilds, err := j.getBuildsFrom(tx, "transition_build_id", jobIDs)
	if err != nil {
		return nil, err
	}

	dashboard := Dashboard{}
	for _, job := range jobs {
		dashboardJob := DashboardJob{Job: job}

		if nextBuild, found := nextBuilds[job.ID()]; found {
			dashboardJob.NextBuild = nextBuild
		}

		if finishedBuild, found := finishedBuilds[job.ID()]; found {
			dashboardJob.FinishedBuild = finishedBuild
		}

		if transitionBuild, found := transitionBuilds[job.ID()]; found {
			dashboardJob.TransitionBuild = transitionBuild
		}

		dashboard = append(dashboard, dashboardJob)
	}

	return dashboard, nil
}

func (j *jobFactory) getBuildsFrom(tx Tx, col string, jobIDs []int) (map[int]Build, error) {

	rows, err := buildsQuery.
		Where(sq.Eq{"j.id": jobIDs}).
		Where(sq.Expr("j." + col + " = b.id")).
		RunWith(tx).
		Query()
	if err != nil {
		return nil, err
	}

	defer Close(rows)

	builds := make(map[int]Build)

	for rows.Next() {
		build := &build{conn: j.conn, lockFactory: j.lockFactory}
		err := scanBuild(build, rows, j.conn.EncryptionStrategy())
		if err != nil {
			return nil, err
		}
		builds[build.JobID()] = build
	}

	return builds, nil
}
