package db

import (
	"database/sql"
	"sort"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/lib/pq"
)

//go:generate counterfeiter . JobFactory

type JobFactory interface {
	VisibleJobs([]string) (atc.Dashboard, error)
	AllActiveJobs() (atc.Dashboard, error)
	JobsToSchedule() (Jobs, error)
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

func (j *jobFactory) VisibleJobs(teamNames []string) (atc.Dashboard, error) {
	tx, err := j.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	dashboard, err := buildDashboard(tx, sq.Or{
		sq.Eq{"tm.name": teamNames},
		sq.Eq{"p.public": true},
	})
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return dashboard, nil
}

func (j *jobFactory) AllActiveJobs() (atc.Dashboard, error) {
	tx, err := j.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	dashboard, err := buildDashboard(tx, nil)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return dashboard, nil
}

func (j *jobFactory) JobsToSchedule() (Jobs, error) {
	rows, err := jobsQuery.
		Where(sq.Expr("schedule_requested > last_scheduled")).
		RunWith(j.conn).
		Query()
	if err != nil {
		return nil, err
	}

	return scanJobs(j.conn, j.lockFactory, rows)
}

func buildDashboard(tx Tx, pred interface{}) (atc.Dashboard, error) {
	rows, err := psql.Select("j.id", "j.name", "p.name", "j.paused", "j.has_new_inputs", "j.tags", "tm.name",
		"l.id", "l.name", "l.status", "l.start_time", "l.end_time",
		"n.id", "n.name", "n.status", "n.start_time", "n.end_time",
		"t.id", "t.name", "t.status", "t.start_time", "t.end_time").
		From("jobs j").
		Join("pipelines p ON j.pipeline_id = p.id").
		Join("teams tm ON p.team_id = tm.id").
		LeftJoin("builds l on j.latest_completed_build_id = l.id").
		LeftJoin("builds n on j.next_build_id = n.id").
		LeftJoin("builds t on j.transition_build_id = t.id").
		Where(sq.Eq{
			"j.active": true,
		}).
		Where(pred).
		OrderBy("j.id ASC").
		RunWith(tx).
		Query()
	if err != nil {
		return nil, err
	}

	type nullableBuild struct {
		id        sql.NullInt64
		name      sql.NullString
		jobName   sql.NullString
		status    sql.NullString
		startTime pq.NullTime
		endTime   pq.NullTime
	}

	var dashboard atc.Dashboard
	for rows.Next() {
		var (
			f, n, t nullableBuild
		)

		j := atc.DashboardJob{}
		err = rows.Scan(&j.ID, &j.Name, &j.PipelineName, &j.Paused, &j.HasNewInputs, pq.Array(&j.Groups), &j.TeamName,
			&f.id, &f.name, &f.status, &f.startTime, &f.endTime,
			&n.id, &n.name, &n.status, &n.startTime, &n.endTime,
			&t.id, &t.name, &t.status, &t.startTime, &t.endTime)
		if err != nil {
			return nil, err
		}

		if f.id.Valid {
			j.FinishedBuild = &atc.DashboardBuild{
				ID:           int(f.id.Int64),
				Name:         f.name.String,
				JobName:      j.Name,
				PipelineName: j.PipelineName,
				TeamName:     j.TeamName,
				Status:       f.status.String,
				StartTime:    f.startTime.Time,
				EndTime:      f.endTime.Time,
			}
		}

		if n.id.Valid {
			j.NextBuild = &atc.DashboardBuild{
				ID:           int(n.id.Int64),
				Name:         n.name.String,
				JobName:      j.Name,
				PipelineName: j.PipelineName,
				TeamName:     j.TeamName,
				Status:       n.status.String,
				StartTime:    n.startTime.Time,
				EndTime:      n.endTime.Time,
			}
		}

		if t.id.Valid {
			j.TransitionBuild = &atc.DashboardBuild{
				ID:           int(t.id.Int64),
				Name:         t.name.String,
				JobName:      j.Name,
				PipelineName: j.PipelineName,
				TeamName:     j.TeamName,
				Status:       t.status.String,
				StartTime:    t.startTime.Time,
				EndTime:      t.endTime.Time,
			}
		}

		dashboard = append(dashboard, j)
	}

	rows, err = psql.Select("j.id", "i.name", "r.name", "array_agg(jp.name)", "i.trigger").
		From("job_inputs i").
		Join("jobs j ON j.id = i.job_id").
		Join("pipelines p ON p.id = j.pipeline_id").
		Join("teams tm ON tm.id = p.team_id").
		Join("resources r ON r.id = i.resource_id").
		LeftJoin("jobs jp ON jp.id = i.passed_job_id").
		Where(sq.Eq{
			"j.active": true,
		}).
		Where(pred).
		GroupBy("i.name, j.id, r.name, i.trigger").
		OrderBy("j.id").
		RunWith(tx).
		Query()
	if err != nil {
		return nil, err
	}

	jobInputs := make(map[int][]atc.DashboardJobInput)
	for rows.Next() {
		var passedString []sql.NullString
		var inputName, resourceName string
		var jobID int
		var trigger bool

		err = rows.Scan(&jobID, &inputName, &resourceName, pq.Array(&passedString), &trigger)
		if err != nil {
			return nil, err
		}

		var passed []string
		for _, s := range passedString {
			if s.Valid {
				passed = append(passed, s.String)
			}
		}

		jobInputs[jobID] = append(jobInputs[jobID], atc.DashboardJobInput{
			Name:     inputName,
			Resource: resourceName,
			Trigger:  trigger,
			Passed:   passed,
		})
	}

	var finalDashboard atc.Dashboard
	for _, job := range dashboard {
		for _, input := range jobInputs[job.ID] {
			job.Inputs = append(job.Inputs, input)
		}

		sort.Slice(job.Inputs, func(p, q int) bool {
			return job.Inputs[p].Name < job.Inputs[q].Name
		})

		finalDashboard = append(finalDashboard, job)
	}

	return finalDashboard, nil
}
