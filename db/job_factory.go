package db

import (
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc/db/lock"
)

//go:generate counterfeiter . JobFactory

type JobFactory interface {
	VisibleJobs([]string) (Dashboard, error)
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
	rows, err := jobsQuery.
		Where(sq.Eq{
			"t.name":   teamNames,
			"j.active": true,
		}).
		OrderBy("j.id ASC").
		RunWith(j.conn).
		Query()
	if err != nil {
		return nil, err
	}

	currentTeamJobs, err := scanJobs(j.conn, j.lockFactory, rows)
	if err != nil {
		return nil, err
	}

	rows, err = jobsQuery.
		Where(sq.NotEq{
			"t.name": teamNames,
		}).
		Where(sq.Eq{
			"p.public": true,
			"j.active": true,
		}).
		OrderBy("j.id ASC").
		RunWith(j.conn).
		Query()
	if err != nil {
		return nil, err
	}

	otherTeamPublicJobs, err := scanJobs(j.conn, j.lockFactory, rows)
	if err != nil {
		return nil, err
	}

	jobs := append(currentTeamJobs, otherTeamPublicJobs...)

	var jobIDs []int
	for _, job := range jobs {
		jobIDs = append(jobIDs, job.ID())
	}

	nextBuilds, err := j.getBuildsFrom("next_builds_per_job", jobIDs)
	if err != nil {
		return nil, err
	}

	finishedBuilds, err := j.getBuildsFrom("latest_completed_builds_per_job", jobIDs)
	if err != nil {
		return nil, err
	}

	transitionBuilds, err := j.getBuildsFrom("transition_builds_per_job", jobIDs)
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

func (j *jobFactory) getBuildsFrom(view string, jobIDs []int) (map[int]Build, error) {
	rows, err := buildsQuery.
		From(view + " b").
		Where(sq.Eq{"j.id": jobIDs}).
		RunWith(j.conn).Query()
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
