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
		}).
		RunWith(tx).
		Query()
	if err != nil {
		return nil, err
	}
	defer Close(rows)

	jobs, err := scanJobs(j.conn, j.lockFactory, rows)
	if err != nil {
		return nil, err
	}

	// Separate paused from non-paused jobs; only non-paused jobs need
	// resource, resource type, and prototype lookups.
	schedulerJobs := make(SchedulerJobs, 0, len(jobs))
	var nonPausedJobs []Job
	var nonPausedJobIDs []int
	for _, job := range jobs {
		if job.Paused() || job.PipelineIsPaused() {
			schedulerJobs = append(schedulerJobs, SchedulerJob{Job: job})
		} else {
			nonPausedJobs = append(nonPausedJobs, job)
			nonPausedJobIDs = append(nonPausedJobIDs, job.ID())
		}
	}

	if len(nonPausedJobs) == 0 {
		err = tx.Commit()
		if err != nil {
			return nil, err
		}
		return schedulerJobs, nil
	}

	jobResources, err := j.fetchJobResources(tx, nonPausedJobIDs)
	if err != nil {
		return nil, err
	}

	pipelineResourceTypes := make(map[int]ResourceTypes)
	pipelinePrototypes := make(map[int]Prototypes)
	for _, job := range nonPausedJobs {
		resourceTypes, found := pipelineResourceTypes[job.PipelineID()]
		if !found {
			resourceTypeRows, err := resourceTypesQuery.
				Where(sq.Eq{"r.pipeline_id": job.PipelineID()}).
				OrderBy("r.name").
				RunWith(tx).
				Query()
			if err != nil {
				return nil, err
			}

			for resourceTypeRows.Next() {
				resourceType := newEmptyResourceType(j.conn, j.lockFactory)
				err := scanResourceType(resourceType, resourceTypeRows)
				if err != nil {
					resourceTypeRows.Close()
					return nil, err
				}

				resourceTypes = append(resourceTypes, resourceType)
			}
			if err = resourceTypeRows.Err(); err != nil {
				resourceTypeRows.Close()
				return nil, err
			}
			resourceTypeRows.Close()

			pipelineResourceTypes[job.PipelineID()] = resourceTypes
		}

		prototypes, found := pipelinePrototypes[job.PipelineID()]
		if !found {
			prototypeRows, err := prototypesQuery.
				Where(sq.Eq{"pt.pipeline_id": job.PipelineID()}).
				OrderBy("pt.name").
				RunWith(tx).
				Query()
			if err != nil {
				return nil, err
			}

			for prototypeRows.Next() {
				prototype := newEmptyPrototype(j.conn, j.lockFactory)
				err := scanPrototype(prototype, prototypeRows)
				if err != nil {
					prototypeRows.Close()
					return nil, err
				}

				prototypes = append(prototypes, prototype)
			}
			if err = prototypeRows.Err(); err != nil {
				prototypeRows.Close()
				return nil, err
			}
			prototypeRows.Close()

			pipelinePrototypes[job.PipelineID()] = prototypes
		}

		schedulerJobs = append(schedulerJobs, SchedulerJob{
			Job:           job,
			Resources:     jobResources[job.ID()],
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

func (j *jobFactory) fetchJobResources(tx Tx, jobIDs []int) (map[int]SchedulerResources, error) {
	rows, err := tx.Query(`
		SELECT sub.job_id, r.name, r.type, r.config, r.nonce
		FROM (
			SELECT ji.job_id, ji.resource_id FROM job_inputs ji WHERE ji.job_id = ANY($1)
			UNION
			SELECT jo.job_id, jo.resource_id FROM job_outputs jo WHERE jo.job_id = ANY($1)
		) sub
		JOIN resources r ON r.id = sub.resource_id`, jobIDs)
	if err != nil {
		return nil, err
	}
	defer Close(rows)

	es := j.conn.EncryptionStrategy()
	result := make(map[int]SchedulerResources)

	for rows.Next() {
		var jobID int
		var name, rsType string
		var configBlob []byte
		var nonce sql.NullString

		err = rows.Scan(&jobID, &name, &rsType, &configBlob, &nonce)
		if err != nil {
			return nil, err
		}

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

		result[jobID] = append(result[jobID], SchedulerResource{
			Name:                 name,
			Type:                 rsType,
			Source:               config.Source,
			ExposeBuildCreatedBy: config.ExposeBuildCreatedBy,
		})
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
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
	pred any

	tx Tx
}

func newDashboardFactory(tx Tx, pred any) dashboardFactory {
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
	defer Close(rows)

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
	defer Close(rows)

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
	defer Close(rows)

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

	return jobOutputs, nil
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
