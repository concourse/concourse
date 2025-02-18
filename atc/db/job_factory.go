package db

import (
	"database/sql"
	"encoding/json"
	"sort"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/jackc/pgx/v5/pgtype"
)

//counterfeiter:generate . JobFactory

// XXX: This job factory object is not really a job factory anymore. It is
// holding the responsibility for two very different things: constructing a
// dashboard object and also a scheduler job object. Figure out what this is
// trying to encapsulate or considering splitting this out!
type JobFactory interface {
	VisibleJobs([]string) ([]atc.JobSummary, error)
	AllActiveJobs() ([]atc.JobSummary, error)
	JobsToSchedule() (SchedulerJobs, error)
}

type jobFactory struct {
	conn        DbConn
	lockFactory lock.LockFactory
}

func NewJobFactory(conn DbConn, lockFactory lock.LockFactory) JobFactory {
	return &jobFactory{
		conn:        conn,
		lockFactory: lockFactory,
	}
}

type SchedulerJobs []SchedulerJob

type SchedulerJob struct {
	Job
	Resources     SchedulerResources
	ResourceTypes atc.ResourceTypes
	Prototypes    atc.Prototypes
}

type SchedulerResources []SchedulerResource

type SchedulerResource struct {
	Name                 string
	Type                 string
	Source               atc.Source
	ExposeBuildCreatedBy bool
}

func (r *SchedulerResource) ApplySourceDefaults(resourceTypes atc.ResourceTypes) {
	parentType, found := resourceTypes.Lookup(r.Type)
	if found {
		r.Source = parentType.Defaults.Merge(r.Source)
	} else {
		defaults, found := atc.FindBaseResourceTypeDefaults(r.Type)
		if found {
			r.Source = defaults.Merge(r.Source)
		}
	}
}

func (resources SchedulerResources) Lookup(name string) (*SchedulerResource, bool) {
	for _, resource := range resources {
		if resource.Name == name {
			return &resource, true
		}
	}

	return nil, false
}

func (j *jobFactory) JobsToSchedule() (SchedulerJobs, error) {
	tx, err := j.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	rows, err := jobsQuery.
		Where(sq.Expr("j.schedule_requested > j.last_scheduled")).
		Where(sq.Eq{
			"j.active": true,
			"j.paused": false,
			"p.paused": false,
		}).
		RunWith(tx).
		Query()
	if err != nil {
		return nil, err
	}

	jobs, err := scanJobs(j.conn, j.lockFactory, rows)
	if err != nil {
		return nil, err
	}

	var schedulerJobs SchedulerJobs
	pipelineResourceTypes := make(map[int]ResourceTypes)
	pipelinePrototypes := make(map[int]Prototypes)
	for _, job := range jobs {
		rows, err := tx.Query(`WITH inputs AS (
				SELECT ji.resource_id from job_inputs ji where ji.job_id = $1
				UNION
				SELECT jo.resource_id from job_outputs jo where jo.job_id = $1
			)
			SELECT r.name, r.type, r.config, r.nonce
			From resources r
			Join inputs i on i.resource_id = r.id`, job.ID())
		if err != nil {
			return nil, err
		}

		var schedulerResources SchedulerResources
		for rows.Next() {
			var name, type_ string
			var configBlob []byte
			var nonce sql.NullString

			err = rows.Scan(&name, &type_, &configBlob, &nonce)
			if err != nil {
				return nil, err
			}

			defer Close(rows)

			es := j.conn.EncryptionStrategy()

			var noncense *string
			if nonce.Valid {
				noncense = &nonce.String
			}

			decryptedConfig, err := es.Decrypt(string(configBlob), noncense)
			if err != nil {
				return nil, err
			}

			var config atc.ResourceConfig
			err = json.Unmarshal(decryptedConfig, &config)
			if err != nil {
				return nil, err
			}

			schedulerResources = append(schedulerResources, SchedulerResource{
				Name:                 name,
				Type:                 type_,
				Source:               config.Source,
				ExposeBuildCreatedBy: config.ExposeBuildCreatedBy,
			})
		}

		resourceTypes, found := pipelineResourceTypes[job.PipelineID()]
		if !found {
			rows, err := resourceTypesQuery.
				Where(sq.Eq{"r.pipeline_id": job.PipelineID()}).
				OrderBy("r.name").
				RunWith(tx).
				Query()
			if err != nil {
				return nil, err
			}

			defer Close(rows)

			for rows.Next() {
				resourceType := newEmptyResourceType(j.conn, j.lockFactory)
				err := scanResourceType(resourceType, rows)
				if err != nil {
					return nil, err
				}

				resourceTypes = append(resourceTypes, resourceType)
			}

			pipelineResourceTypes[job.PipelineID()] = resourceTypes
		}

		prototypes, found := pipelinePrototypes[job.PipelineID()]
		if !found {
			rows, err := prototypesQuery.
				Where(sq.Eq{"pt.pipeline_id": job.PipelineID()}).
				OrderBy("pt.name").
				RunWith(tx).
				Query()
			if err != nil {
				return nil, err
			}

			defer Close(rows)

			for rows.Next() {
				prototype := newEmptyPrototype(j.conn, j.lockFactory)
				err := scanPrototype(prototype, rows)
				if err != nil {
					return nil, err
				}

				prototypes = append(prototypes, prototype)
			}

			pipelinePrototypes[job.PipelineID()] = prototypes
		}

		schedulerJobs = append(schedulerJobs, SchedulerJob{
			Job:           job,
			Resources:     schedulerResources,
			ResourceTypes: resourceTypes.Deserialize(),
			Prototypes:    prototypes.Configs(),
		})
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return schedulerJobs, nil
}

func (j *jobFactory) VisibleJobs(teamNames []string) ([]atc.JobSummary, error) {
	tx, err := j.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	dashboardFactory := newDashboardFactory(tx, sq.Or{
		sq.Eq{"tm.name": teamNames},
		sq.Eq{"p.public": true},
	})

	dashboard, err := dashboardFactory.buildDashboard()
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return dashboard, nil
}

func (j *jobFactory) AllActiveJobs() ([]atc.JobSummary, error) {
	tx, err := j.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	dashboardFactory := newDashboardFactory(tx, nil)
	dashboard, err := dashboardFactory.buildDashboard()
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return dashboard, nil
}

type dashboardFactory struct {
	// Constraints that are used by the dashboard queries. For example, a job ID
	// constraint so that the dashboard will only return the job I have access to
	// see.
	pred interface{}

	tx Tx
}

func newDashboardFactory(tx Tx, pred interface{}) dashboardFactory {
	return dashboardFactory{
		pred: pred,
		tx:   tx,
	}
}

func (d dashboardFactory) buildDashboard() ([]atc.JobSummary, error) {
	dashboard, err := d.constructJobsForDashboard()
	if err != nil {
		return nil, err
	}

	jobInputs, err := d.fetchJobInputs()
	if err != nil {
		return nil, err
	}

	jobOutputs, err := d.fetchJobOutputs()
	if err != nil {
		return nil, err
	}

	return d.combineJobInputsAndOutputsWithDashboardJobs(dashboard, jobInputs, jobOutputs), nil
}

func (d dashboardFactory) constructJobsForDashboard() ([]atc.JobSummary, error) {
	rows, err := psql.Select(
		"j.id",
		"j.name",
		"p.id",
		"p.name",
		"p.instance_vars",
		"j.paused",
		"j.has_new_inputs",
		"j.tags",
		"tm.name",
		"l.id", "l.name", "l.status", "l.start_time", "l.end_time",
		"n.id", "n.name", "n.status", "n.start_time", "n.end_time",
		"t.id", "t.name", "t.status", "t.start_time", "t.end_time",
		"j.paused_by",
		"j.paused_at").
		From("jobs j").
		Join("pipelines p ON j.pipeline_id = p.id").
		Join("teams tm ON p.team_id = tm.id").
		LeftJoin("builds l on j.latest_completed_build_id = l.id").
		LeftJoin("builds n on j.next_build_id = n.id").
		LeftJoin("builds t on j.transition_build_id = t.id").
		Where(sq.Eq{
			"j.active": true,
		}).
		Where(d.pred).
		OrderBy("j.id ASC").
		RunWith(d.tx).
		Query()
	if err != nil {
		return nil, err
	}

	type nullableBuild struct {
		id        sql.NullInt64
		name      sql.NullString
		status    sql.NullString
		startTime sql.NullTime
		endTime   sql.NullTime
	}

	var dashboard []atc.JobSummary
	for rows.Next() {
		var (
			f, n, t              nullableBuild
			jobPausedBy          sql.NullString
			jobPausedAt          sql.NullTime
			pipelineInstanceVars sql.NullString
		)

		j := atc.JobSummary{}
		m := pgtype.NewMap()
		err = rows.Scan(&j.ID, &j.Name, &j.PipelineID, &j.PipelineName, &pipelineInstanceVars, &j.Paused, &j.HasNewInputs, m.SQLScanner(&j.Groups), &j.TeamName,
			&f.id, &f.name, &f.status, &f.startTime, &f.endTime,
			&n.id, &n.name, &n.status, &n.startTime, &n.endTime,
			&t.id, &t.name, &t.status, &t.startTime, &t.endTime,
			&jobPausedBy, &jobPausedAt)
		if err != nil {
			return nil, err
		}

		if jobPausedBy.Valid {
			j.PausedBy = jobPausedBy.String
		}

		if jobPausedAt.Valid {
			j.PausedAt = jobPausedAt.Time.Unix()
		}

		if pipelineInstanceVars.Valid {
			err = json.Unmarshal([]byte(pipelineInstanceVars.String), &j.PipelineInstanceVars)
			if err != nil {
				return nil, err
			}
		}

		if f.id.Valid {
			j.FinishedBuild = &atc.BuildSummary{
				ID:                   int(f.id.Int64),
				Name:                 f.name.String,
				JobName:              j.Name,
				PipelineID:           j.PipelineID,
				PipelineName:         j.PipelineName,
				PipelineInstanceVars: j.PipelineInstanceVars,
				TeamName:             j.TeamName,
				Status:               atc.BuildStatus(f.status.String),
				StartTime:            f.startTime.Time.Unix(),
				EndTime:              f.endTime.Time.Unix(),
			}
		}

		if n.id.Valid {
			j.NextBuild = &atc.BuildSummary{
				ID:                   int(n.id.Int64),
				Name:                 n.name.String,
				JobName:              j.Name,
				PipelineID:           j.PipelineID,
				PipelineName:         j.PipelineName,
				PipelineInstanceVars: j.PipelineInstanceVars,
				TeamName:             j.TeamName,
				Status:               atc.BuildStatus(n.status.String),
				StartTime:            n.startTime.Time.Unix(),
				EndTime:              n.endTime.Time.Unix(),
			}
		}

		if t.id.Valid {
			j.TransitionBuild = &atc.BuildSummary{
				ID:                   int(t.id.Int64),
				Name:                 t.name.String,
				JobName:              j.Name,
				PipelineID:           j.PipelineID,
				PipelineName:         j.PipelineName,
				PipelineInstanceVars: j.PipelineInstanceVars,
				TeamName:             j.TeamName,
				Status:               atc.BuildStatus(t.status.String),
				StartTime:            t.startTime.Time.Unix(),
				EndTime:              t.endTime.Time.Unix(),
			}
		}

		dashboard = append(dashboard, j)
	}

	return dashboard, nil
}

func (d dashboardFactory) fetchJobInputs() (map[int][]atc.JobInputSummary, error) {
	rows, err := psql.Select("j.id", "i.name", "r.name", "array_remove(array_agg(jp.name ORDER BY jp.id), NULL) passed", "i.trigger").
		From("job_inputs i").
		Join("jobs j ON j.id = i.job_id").
		Join("pipelines p ON p.id = j.pipeline_id").
		Join("teams tm ON tm.id = p.team_id").
		Join("resources r ON r.id = i.resource_id").
		LeftJoin("jobs jp ON jp.id = i.passed_job_id").
		Where(sq.Eq{
			"j.active": true,
		}).
		Where(d.pred).
		GroupBy("i.name, j.id, r.name, i.trigger").
		OrderBy("j.id").
		RunWith(d.tx).
		Query()
	if err != nil {
		return nil, err
	}

	jobInputs := make(map[int][]atc.JobInputSummary)
	m := pgtype.NewMap()
	for rows.Next() {
		var passed []string
		var inputName, resourceName string
		var jobID int
		var trigger bool

		err = rows.Scan(&jobID, &inputName, &resourceName, m.SQLScanner(&passed), &trigger)
		if err != nil {
			return nil, err
		}

		if len(passed) == 0 {
			passed = nil
		}

		jobInputs[jobID] = append(jobInputs[jobID], atc.JobInputSummary{
			Name:     inputName,
			Resource: resourceName,
			Trigger:  trigger,
			Passed:   passed,
		})
	}

	return jobInputs, nil
}

func (d dashboardFactory) fetchJobOutputs() (map[int][]atc.JobOutputSummary, error) {
	rows, err := psql.Select("o.name", "r.name", "o.job_id").
		From("job_outputs o").
		Join("jobs j ON j.id = o.job_id").
		Join("pipelines p ON p.id = j.pipeline_id").
		Join("teams tm ON tm.id = p.team_id").
		Join("resources r ON r.id = o.resource_id").
		Where(d.pred).
		Where(sq.Eq{
			"j.active": true,
		}).
		OrderBy("j.id").
		RunWith(d.tx).
		Query()
	if err != nil {
		return nil, err
	}

	jobOutputs := make(map[int][]atc.JobOutputSummary)
	for rows.Next() {
		var output atc.JobOutputSummary
		var jobID int
		err = rows.Scan(&output.Name, &output.Resource, &jobID)
		if err != nil {
			return nil, err
		}

		jobOutputs[jobID] = append(jobOutputs[jobID], output)
	}

	return jobOutputs, err
}

func (d dashboardFactory) combineJobInputsAndOutputsWithDashboardJobs(dashboard []atc.JobSummary, jobInputs map[int][]atc.JobInputSummary, jobOutputs map[int][]atc.JobOutputSummary) []atc.JobSummary {
	var finalDashboard []atc.JobSummary
	for _, job := range dashboard {
		job.Inputs = append(job.Inputs, jobInputs[job.ID]...)

		sort.Slice(job.Inputs, func(p, q int) bool {
			return job.Inputs[p].Name < job.Inputs[q].Name
		})

		job.Outputs = append(job.Outputs, jobOutputs[job.ID]...)

		sort.Slice(job.Outputs, func(p, q int) bool {
			return job.Outputs[p].Name < job.Outputs[q].Name
		})

		finalDashboard = append(finalDashboard, job)
	}

	return finalDashboard
}
