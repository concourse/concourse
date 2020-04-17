package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"code.cloudfoundry.org/lager"

	sq "github.com/Masterminds/squirrel"
	"github.com/lib/pq"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/event"
)

var ErrConfigComparisonFailed = errors.New("comparison with existing config failed during save")

//go:generate counterfeiter . Team

type Team interface {
	ID() int
	Name() string
	Admin() bool

	Auth() atc.TeamAuth

	Delete() error
	Rename(string) error

	SavePipeline(
		pipelineName string,
		config atc.Config,
		from ConfigVersion,
		initiallyPaused bool,
	) (Pipeline, bool, error)

	Pipeline(pipelineName string) (Pipeline, bool, error)
	Pipelines() ([]Pipeline, error)
	PublicPipelines() ([]Pipeline, error)
	OrderPipelines([]string) error

	CreateOneOffBuild() (Build, error)
	CreateStartedBuild(plan atc.Plan) (Build, error)

	PrivateAndPublicBuilds(Page) ([]Build, Pagination, error)
	Builds(page Page) ([]Build, Pagination, error)
	BuildsWithTime(page Page) ([]Build, Pagination, error)

	SaveWorker(atcWorker atc.Worker, ttl time.Duration) (Worker, error)
	Workers() ([]Worker, error)
	FindVolumeForWorkerArtifact(int) (CreatedVolume, bool, error)

	Containers() ([]Container, error)
	IsCheckContainer(string) (bool, error)
	IsContainerWithinTeam(string, bool) (bool, error)

	FindContainerByHandle(string) (Container, bool, error)
	FindCheckContainers(lager.Logger, string, string, creds.Secrets, creds.VarSourcePool) ([]Container, map[int]time.Time, error)
	FindContainersByMetadata(ContainerMetadata) ([]Container, error)
	FindCreatedContainerByHandle(string) (CreatedContainer, bool, error)
	FindWorkerForContainer(handle string) (Worker, bool, error)
	FindWorkerForVolume(handle string) (Worker, bool, error)

	UpdateProviderAuth(auth atc.TeamAuth) error
}

type team struct {
	id          int
	conn        Conn
	lockFactory lock.LockFactory

	name  string
	admin bool

	auth atc.TeamAuth
}

func (t *team) ID() int      { return t.id }
func (t *team) Name() string { return t.name }
func (t *team) Admin() bool  { return t.admin }

func (t *team) Auth() atc.TeamAuth { return t.auth }

func (t *team) Delete() error {
	_, err := psql.Delete("teams").
		Where(sq.Eq{
			"name": t.name,
		}).
		RunWith(t.conn).
		Exec()

	return err
}

func (t *team) Rename(name string) error {
	_, err := psql.Update("teams").
		Set("name", name).
		Where(sq.Eq{
			"id": t.id,
		}).
		RunWith(t.conn).
		Exec()

	return err
}

func (t *team) Workers() ([]Worker, error) {
	return getWorkers(t.conn, workersQuery.Where(sq.Or{
		sq.Eq{"t.id": t.id},
		sq.Eq{"w.team_id": nil},
	}))
}

func (t *team) FindVolumeForWorkerArtifact(artifactID int) (CreatedVolume, bool, error) {
	tx, err := t.conn.Begin()
	if err != nil {
		return nil, false, err
	}

	defer Rollback(tx)

	artifact, found, err := getWorkerArtifact(tx, t.conn, artifactID)
	if err != nil {
		return nil, false, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	return artifact.Volume(t.ID())
}

func (t *team) FindWorkerForContainer(handle string) (Worker, bool, error) {
	return getWorker(t.conn, workersQuery.Join("containers c ON c.worker_name = w.name").Where(sq.And{
		sq.Eq{"c.handle": handle},
	}))
}

func (t *team) FindWorkerForVolume(handle string) (Worker, bool, error) {
	return getWorker(t.conn, workersQuery.Join("volumes v ON v.worker_name = w.name").Where(sq.And{
		sq.Eq{"v.handle": handle},
	}))
}

func (t *team) Containers() ([]Container, error) {
	rows, err := selectContainers("c").
		Join("workers w ON c.worker_name = w.name").
		Join("resource_config_check_sessions rccs ON rccs.id = c.resource_config_check_session_id").
		Join("resources r ON r.resource_config_id = rccs.resource_config_id").
		Join("pipelines p ON p.id = r.pipeline_id").
		Where(sq.Eq{
			"p.team_id": t.id,
		}).
		Where(sq.Or{
			sq.Eq{
				"w.team_id": t.id,
			}, sq.Eq{
				"w.team_id": nil,
			},
		}).
		Distinct().
		RunWith(t.conn).
		Query()
	if err != nil {
		return nil, err
	}

	var containers []Container
	containers, err = scanContainers(rows, t.conn, containers)
	if err != nil {
		return nil, err
	}

	rows, err = selectContainers("c").
		Join("workers w ON c.worker_name = w.name").
		Join("resource_config_check_sessions rccs ON rccs.id = c.resource_config_check_session_id").
		Join("resource_types rt ON rt.resource_config_id = rccs.resource_config_id").
		Join("pipelines p ON p.id = rt.pipeline_id").
		Where(sq.Eq{
			"p.team_id": t.id,
		}).
		Where(sq.Or{
			sq.Eq{
				"w.team_id": t.id,
			}, sq.Eq{
				"w.team_id": nil,
			},
		}).
		Distinct().
		RunWith(t.conn).
		Query()
	if err != nil {
		return nil, err
	}

	containers, err = scanContainers(rows, t.conn, containers)
	if err != nil {
		return nil, err
	}

	rows, err = selectContainers("c").
		Where(sq.Eq{
			"c.team_id": t.id,
		}).
		RunWith(t.conn).
		Query()
	if err != nil {
		return nil, err
	}

	containers, err = scanContainers(rows, t.conn, containers)
	if err != nil {
		return nil, err
	}

	return containers, nil
}

func (t *team) IsCheckContainer(handle string) (bool, error) {
	var containerType string
	err := psql.Select("meta_type").
		From("containers").
		Where(sq.Eq{
			"handle": handle,
		}).
		RunWith(t.conn).
		QueryRow().
		Scan(&containerType)
	if err != nil {
		return false, err
	}

	return ContainerType(containerType) == ContainerTypeCheck, nil
}

func (t *team) IsContainerWithinTeam(handle string, isCheck bool) (bool, error) {
	var ok int
	var err error

	if isCheck {
		err = psql.Select("1").
			From("resources r").
			Join("pipelines p ON p.id = r.pipeline_id").
			Join("resource_configs rc ON rc.id = r.resource_config_id").
			Join("resource_config_check_sessions rccs ON rccs.resource_config_id = rc.id").
			Join("containers c ON rccs.id = c.resource_config_check_session_id").
			Where(sq.Eq{
				"c.handle":  handle,
				"p.team_id": t.id,
			}).
			RunWith(t.conn).
			QueryRow().
			Scan(&ok)
	} else {
		err = psql.Select("1").
			From("containers c").
			Where(sq.Eq{
				"c.team_id": t.id,
				"c.handle":  handle,
			}).
			RunWith(t.conn).
			QueryRow().
			Scan(&ok)
	}
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (t *team) FindContainerByHandle(
	handle string,
) (Container, bool, error) {
	creatingContainer, createdContainer, err := t.findContainer(sq.Eq{"handle": handle})
	if err != nil {
		return nil, false, err
	}

	if creatingContainer != nil {
		return creatingContainer, true, nil
	}

	if createdContainer != nil {
		return createdContainer, true, nil
	}

	return nil, false, nil
}

func (t *team) FindContainersByMetadata(metadata ContainerMetadata) ([]Container, error) {
	eq := sq.Eq(metadata.SQLMap())
	eq["team_id"] = t.id

	rows, err := selectContainers().
		Where(eq).
		RunWith(t.conn).
		Query()
	if err != nil {
		return nil, err
	}

	var containers []Container

	containers, err = scanContainers(rows, t.conn, containers)
	if err != nil {
		return nil, err
	}

	return containers, nil
}

func (t *team) FindCreatedContainerByHandle(
	handle string,
) (CreatedContainer, bool, error) {
	_, createdContainer, err := t.findContainer(sq.Eq{"handle": handle})
	if err != nil {
		return nil, false, err
	}

	if createdContainer != nil {
		return createdContainer, true, nil
	}

	return nil, false, nil
}

func (t *team) SavePipeline(
	pipelineName string,
	config atc.Config,
	from ConfigVersion,
	initiallyPaused bool,
) (Pipeline, bool, error) {
	tx, err := t.conn.Begin()
	if err != nil {
		return nil, false, err
	}

	defer Rollback(tx)

	var existingConfig bool
	err = tx.QueryRow(`SELECT EXISTS (
		SELECT 1
		FROM pipelines
		WHERE name = $1
		AND team_id = $2
	)`, pipelineName, t.id).Scan(&existingConfig)
	if err != nil {
		return nil, false, err
	}

	groupsPayload, err := json.Marshal(config.Groups)
	if err != nil {
		return nil, false, err
	}

	varSourcesPayload, err := json.Marshal(config.VarSources)
	if err != nil {
		return nil, false, err
	}

	encryptedVarSourcesPayload, nonce, err := t.conn.EncryptionStrategy().Encrypt(varSourcesPayload)
	if err != nil {
		return nil, false, err
	}

	var pipelineID int
	if !existingConfig {
		err = psql.Insert("pipelines").
			SetMap(map[string]interface{}{
				"name":         pipelineName,
				"groups":       groupsPayload,
				"var_sources":  encryptedVarSourcesPayload,
				"nonce":        nonce,
				"version":      sq.Expr("nextval('config_version_seq')"),
				"ordering":     sq.Expr("currval('pipelines_id_seq')"),
				"paused":       initiallyPaused,
				"last_updated": sq.Expr("now()"),
				"team_id":      t.id,
			}).
			Suffix("RETURNING id").
			RunWith(tx).
			QueryRow().Scan(&pipelineID)
		if err != nil {
			return nil, false, err
		}
	} else {
		err := psql.Update("pipelines").
			Set("archived", false).
			Set("groups", groupsPayload).
			Set("var_sources", encryptedVarSourcesPayload).
			Set("nonce", nonce).
			Set("version", sq.Expr("nextval('config_version_seq')")).
			Set("last_updated", sq.Expr("now()")).
			Where(sq.Eq{
				"name":    pipelineName,
				"version": from,
				"team_id": t.id,
			}).
			Suffix("RETURNING id").
			RunWith(tx).
			QueryRow().
			Scan(&pipelineID)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, false, ErrConfigComparisonFailed
			}

			return nil, false, err
		}

		err = t.resetDependentTableStates(tx, pipelineID)
		if err != nil {
			return nil, false, err
		}
	}

	resourceNameToID, err := t.saveResources(tx, config.Resources, pipelineID)
	if err != nil {
		return nil, false, err
	}

	_, err = psql.Update("resources").
		Set("resource_config_id", nil).
		Where(sq.Eq{
			"pipeline_id": pipelineID,
			"active":      false,
		}).
		RunWith(tx).
		Exec()
	if err != nil {
		return nil, false, err
	}

	err = t.saveResourceTypes(tx, config.ResourceTypes, pipelineID)
	if err != nil {
		return nil, false, err
	}

	err = t.updateName(tx, config.Jobs, pipelineID)
	if err != nil {
		return nil, false, err
	}

	jobNameToID, err := t.saveJobsAndSerialGroups(tx, config.Jobs, config.Groups, pipelineID)
	if err != nil {
		return nil, false, err
	}

	err = removeUnusedWorkerTaskCaches(tx, pipelineID, config.Jobs)
	if err != nil {
		return nil, false, err
	}

	err = t.insertJobPipes(tx, config.Jobs, resourceNameToID, jobNameToID, pipelineID)
	if err != nil {
		return nil, false, err
	}

	pipeline := newPipeline(t.conn, t.lockFactory)
	err = scanPipeline(
		pipeline,
		pipelinesQuery.
			Where(sq.Eq{"p.id": pipelineID}).
			RunWith(tx).
			QueryRow(),
	)
	if err != nil {
		return nil, false, err
	}

	err = requestScheduleForJobsInPipeline(tx, pipelineID)
	if err != nil {
		return nil, false, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, false, err
	}

	return pipeline, !existingConfig, nil
}

func (t *team) Pipeline(pipelineName string) (Pipeline, bool, error) {
	pipeline := newPipeline(t.conn, t.lockFactory)

	err := scanPipeline(
		pipeline,
		pipelinesQuery.
			Where(sq.Eq{
				"p.team_id": t.id,
				"p.name":    pipelineName,
			}).
			RunWith(t.conn).
			QueryRow(),
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	return pipeline, true, nil
}

func (t *team) Pipelines() ([]Pipeline, error) {
	rows, err := pipelinesQuery.
		Where(sq.Eq{
			"team_id": t.id,
		}).
		OrderBy("ordering").
		RunWith(t.conn).
		Query()
	if err != nil {
		return nil, err
	}

	pipelines, err := scanPipelines(t.conn, t.lockFactory, rows)
	if err != nil {
		return nil, err
	}

	return pipelines, nil
}

func (t *team) PublicPipelines() ([]Pipeline, error) {
	rows, err := pipelinesQuery.
		Where(sq.Eq{
			"team_id": t.id,
			"public":  true,
		}).
		OrderBy("team_id ASC", "ordering ASC").
		RunWith(t.conn).
		Query()
	if err != nil {
		return nil, err
	}

	pipelines, err := scanPipelines(t.conn, t.lockFactory, rows)
	if err != nil {
		return nil, err
	}

	return pipelines, nil
}

func (t *team) OrderPipelines(pipelineNames []string) error {
	tx, err := t.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	for i, name := range pipelineNames {
		pipelineUpdate, err := psql.Update("pipelines").
			Set("ordering", i).
			Where(sq.Eq{
				"name":    name,
				"team_id": t.id,
			}).
			RunWith(tx).
			Exec()
		if err != nil {
			return err
		}
		updatedPipelines, err := pipelineUpdate.RowsAffected()
		if err != nil {
			return err
		}
		if updatedPipelines == 0 {
			return fmt.Errorf("pipeline %s does not exist", name)
		}
	}

	return tx.Commit()
}

// XXX: This is only begin used by tests, replace all tests to CreateBuild on a job
func (t *team) CreateOneOffBuild() (Build, error) {
	tx, err := t.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	build := newEmptyBuild(t.conn, t.lockFactory)
	err = createBuild(tx, build, map[string]interface{}{
		"name":    sq.Expr("nextval('one_off_name')"),
		"team_id": t.id,
		"status":  BuildStatusPending,
	})
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return build, nil
}

func (t *team) CreateStartedBuild(plan atc.Plan) (Build, error) {
	tx, err := t.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	metadata, err := json.Marshal(plan)
	if err != nil {
		return nil, err
	}

	encryptedPlan, nonce, err := t.conn.EncryptionStrategy().Encrypt(metadata)
	if err != nil {
		return nil, err
	}

	build := newEmptyBuild(t.conn, t.lockFactory)
	err = createBuild(tx, build, map[string]interface{}{
		"name":         sq.Expr("nextval('one_off_name')"),
		"team_id":      t.id,
		"status":       BuildStatusStarted,
		"start_time":   sq.Expr("now()"),
		"schema":       schema,
		"private_plan": encryptedPlan,
		"public_plan":  plan.Public(),
		"nonce":        nonce,
	})
	if err != nil {
		return nil, err
	}

	err = build.saveEvent(tx, event.Status{
		Status: atc.StatusStarted,
		Time:   build.StartTime().Unix(),
	})
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	if err = t.conn.Bus().Notify(buildStartedChannel()); err != nil {
		return nil, err
	}

	if err = t.conn.Bus().Notify(buildEventsChannel(build.id)); err != nil {
		return nil, err
	}

	return build, nil
}

func (t *team) PrivateAndPublicBuilds(page Page) ([]Build, Pagination, error) {
	newBuildsQuery := buildsQuery.
		Where(sq.Or{sq.Eq{"p.public": true}, sq.Eq{"t.id": t.id}})

	return getBuildsWithPagination(newBuildsQuery, minMaxIdQuery, page, t.conn, t.lockFactory)
}

func (t *team) BuildsWithTime(page Page) ([]Build, Pagination, error) {
	return getBuildsWithDates(buildsQuery.Where(sq.Eq{"t.id": t.id}), minMaxIdQuery, page, t.conn, t.lockFactory)
}

func (t *team) Builds(page Page) ([]Build, Pagination, error) {
	return getBuildsWithPagination(buildsQuery.Where(sq.Eq{"t.id": t.id}), minMaxIdQuery, page, t.conn, t.lockFactory)
}

func (t *team) SaveWorker(atcWorker atc.Worker, ttl time.Duration) (Worker, error) {
	tx, err := t.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	savedWorker, err := saveWorker(tx, atcWorker, &t.id, ttl, t.conn)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return savedWorker, nil
}

func (t *team) UpdateProviderAuth(auth atc.TeamAuth) error {
	tx, err := t.conn.Begin()
	if err != nil {
		return err
	}
	defer Rollback(tx)

	jsonEncodedProviderAuth, err := json.Marshal(auth)
	if err != nil {
		return err
	}

	query := `
		UPDATE teams
		SET auth = $1, legacy_auth = NULL, nonce = NULL
		WHERE id = $2
		RETURNING id, name, admin, auth, nonce
	`
	err = t.queryTeam(tx, query, jsonEncodedProviderAuth, t.id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (t *team) FindCheckContainers(logger lager.Logger, pipelineName string, resourceName string, secretManager creds.Secrets, varSourcePool creds.VarSourcePool) ([]Container, map[int]time.Time, error) {
	pipeline, found, err := t.Pipeline(pipelineName)
	if err != nil {
		return nil, nil, err
	}
	if !found {
		return nil, nil, nil
	}

	resource, found, err := pipeline.Resource(resourceName)
	if err != nil {
		return nil, nil, err
	}
	if !found {
		return nil, nil, nil
	}

	pipelineResourceTypes, err := pipeline.ResourceTypes()
	if err != nil {
		return nil, nil, err
	}

	variables, err := pipeline.Variables(logger, secretManager, varSourcePool)
	if err != nil {
		return nil, nil, err
	}

	versionedResourceTypes := pipelineResourceTypes.Deserialize()

	source, err := creds.NewSource(variables, resource.Source()).Evaluate()
	if err != nil {
		return nil, nil, err
	}

	resourceTypes, err := creds.NewVersionedResourceTypes(variables, versionedResourceTypes).Evaluate()
	if err != nil {
		return nil, nil, err
	}

	resourceConfigFactory := NewResourceConfigFactory(t.conn, t.lockFactory)
	resourceConfig, err := resourceConfigFactory.FindOrCreateResourceConfig(
		resource.Type(),
		source,
		resourceTypes,
	)
	if err != nil {
		return nil, nil, err
	}

	rows, err := selectContainers("c").
		Join("resource_config_check_sessions rccs ON rccs.id = c.resource_config_check_session_id").
		Where(sq.Eq{
			"rccs.resource_config_id": resourceConfig.ID(),
		}).
		Distinct().
		RunWith(t.conn).
		Query()
	if err != nil {
		return nil, nil, err
	}

	var containers []Container

	containers, err = scanContainers(rows, t.conn, containers)
	if err != nil {
		return nil, nil, err
	}

	rows, err = psql.Select("c.id", "rccs.expires_at").
		From("containers c").
		Join("resource_config_check_sessions rccs ON rccs.id = c.resource_config_check_session_id").
		Where(sq.Eq{
			"rccs.resource_config_id": resourceConfig.ID(),
		}).
		Distinct().
		RunWith(t.conn).
		Query()
	if err != nil {
		return nil, nil, err
	}

	defer Close(rows)

	checkContainersExpiresAt := make(map[int]time.Time)
	for rows.Next() {
		var (
			id        int
			expiresAt pq.NullTime
		)

		err = rows.Scan(&id, &expiresAt)
		if err != nil {
			return nil, nil, err
		}

		checkContainersExpiresAt[id] = expiresAt.Time
	}

	return containers, checkContainersExpiresAt, nil
}

type UpdateName struct {
	OldName string
	NewName string
}

func (t *team) updateName(tx Tx, jobs []atc.JobConfig, pipelineID int) error {
	jobsToUpdate := []UpdateName{}

	for _, job := range jobs {
		if job.OldName != "" {
			var count int
			err := psql.Select("COUNT(*) as count").
				From("jobs").
				Where(sq.Eq{
					"name":        job.OldName,
					"pipeline_id": pipelineID}).
				RunWith(tx).
				QueryRow().
				Scan(&count)
			if err != nil {
				return err
			}

			if count != 0 {
				jobsToUpdate = append(jobsToUpdate, UpdateName{
					OldName: job.OldName,
					NewName: job.Name,
				})
			}
		}
	}

	newMap := make(map[int]bool)
	for _, updateNames := range jobsToUpdate {
		isCyclic := checkCyclic(jobsToUpdate, updateNames.OldName, newMap)
		if isCyclic {
			return errors.New("job name swapping is not supported at this time")
		}
	}

	jobsToUpdate = sortUpdateNames(jobsToUpdate)

	for _, updateName := range jobsToUpdate {
		_, err := psql.Delete("jobs").
			Where(sq.Eq{
				"name":        updateName.NewName,
				"pipeline_id": pipelineID,
				"active":      false}).
			RunWith(tx).
			Exec()
		if err != nil {
			return err
		}

		_, err = psql.Update("jobs").
			Set("name", updateName.NewName).
			Where(sq.Eq{"name": updateName.OldName, "pipeline_id": pipelineID}).
			RunWith(tx).
			Exec()
		if err != nil {
			return err
		}
	}

	return nil
}

func checkCyclic(jobNames []UpdateName, curr string, visited map[int]bool) bool {
	for i, job := range jobNames {
		if job.NewName == curr && !visited[i] {
			visited[i] = true
			checkCyclic(jobNames, job.OldName, visited)
		} else if job.NewName == curr && visited[i] && curr != job.OldName {
			return true
		}
	}

	return false
}

func sortUpdateNames(jobNames []UpdateName) []UpdateName {
	newMap := make(map[string]int)
	for i, job := range jobNames {
		newMap[job.NewName] = i + 1

		if newMap[job.OldName] != 0 {
			index := newMap[job.OldName] - 1

			tempJob := jobNames[index]
			jobNames[index] = job
			jobNames[i] = tempJob

			return sortUpdateNames(jobNames)
		}
	}

	return jobNames
}

func (t *team) saveJob(tx Tx, job atc.JobConfig, pipelineID int, groups []string) (int, error) {
	configPayload, err := json.Marshal(job)
	if err != nil {
		return 0, err
	}

	es := t.conn.EncryptionStrategy()
	encryptedPayload, nonce, err := es.Encrypt(configPayload)
	if err != nil {
		return 0, err
	}

	var jobID int
	err = psql.Insert("jobs").
		Columns("name", "pipeline_id", "config", "public", "max_in_flight", "interruptible", "active", "nonce", "tags").
		Values(job.Name, pipelineID, encryptedPayload, job.Public, job.MaxInFlight(), job.Interruptible, true, nonce, pq.Array(groups)).
		Suffix("ON CONFLICT (name, pipeline_id) DO UPDATE SET config = EXCLUDED.config, public = EXCLUDED.public, max_in_flight = EXCLUDED.max_in_flight, interruptible = EXCLUDED.interruptible, active = EXCLUDED.active, nonce = EXCLUDED.nonce, tags = EXCLUDED.tags").
		Suffix("RETURNING id").
		RunWith(tx).
		QueryRow().
		Scan(&jobID)
	if err != nil {
		return 0, err
	}

	return jobID, nil
}

func (t *team) registerSerialGroup(tx Tx, serialGroup string, jobID int) error {
	_, err := psql.Insert("jobs_serial_groups").
		Columns("serial_group", "job_id").
		Values(serialGroup, jobID).
		RunWith(tx).
		Exec()
	return err
}

func (t *team) saveResource(tx Tx, resource atc.ResourceConfig, pipelineID int) (int, error) {
	configPayload, err := json.Marshal(resource)
	if err != nil {
		return 0, err
	}

	es := t.conn.EncryptionStrategy()
	encryptedPayload, nonce, err := es.Encrypt(configPayload)
	if err != nil {
		return 0, err
	}

	var resourceID int
	err = psql.Insert("resources").
		Columns("name", "pipeline_id", "config", "active", "nonce", "type").
		Values(resource.Name, pipelineID, encryptedPayload, true, nonce, resource.Type).
		Suffix("ON CONFLICT (name, pipeline_id) DO UPDATE SET config = EXCLUDED.config, active = EXCLUDED.active, nonce = EXCLUDED.nonce, type = EXCLUDED.type").
		Suffix("RETURNING id").
		RunWith(tx).
		QueryRow().
		Scan(&resourceID)
	if err != nil {
		return 0, err
	}

	if resource.Version != nil {
		_, err = psql.Delete("resource_pins").
			Where(sq.Eq{
				"resource_id": resourceID,
			}).
			RunWith(tx).
			Exec()
		if err != nil {
			return 0, err
		}
	}

	return resourceID, nil
}

func (t *team) saveResourceType(tx Tx, resourceType atc.ResourceType, pipelineID int) error {
	configPayload, err := json.Marshal(resourceType)
	if err != nil {
		return err
	}

	es := t.conn.EncryptionStrategy()
	encryptedPayload, nonce, err := es.Encrypt(configPayload)
	if err != nil {
		return err
	}

	_, err = psql.Insert("resource_types").
		Columns("name", "pipeline_id", "config", "active", "nonce", "type").
		Values(resourceType.Name, pipelineID, encryptedPayload, true, nonce, resourceType.Type).
		Suffix("ON CONFLICT (name, pipeline_id) DO UPDATE SET config = EXCLUDED.config, active = EXCLUDED.active, nonce = EXCLUDED.nonce, type = EXCLUDED.type").
		RunWith(tx).
		Exec()

	return err
}

func checkIfRowsUpdated(tx Tx, query string, params ...interface{}) (bool, error) {
	result, err := tx.Exec(query, params...)
	if err != nil {
		return false, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	if rows == 0 {
		return false, nil
	}

	return true, nil
}

func swallowUniqueiolation(err error) error {
	if err != nil {
		if pgErr, ok := err.(*pq.Error); ok {
			if pgErr.Code.Class().Name() == "integrity_constraint_violation" {
				return nil
			}
		}

		return err
	}

	return nil
}

func (t *team) findContainer(whereClause sq.Sqlizer) (CreatingContainer, CreatedContainer, error) {
	creating, created, destroying, _, err := scanContainer(
		selectContainers().
			Where(whereClause).
			RunWith(t.conn).
			QueryRow(),
		t.conn,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, nil
		}
		return nil, nil, err
	}

	if destroying != nil {
		return nil, nil, nil
	}

	return creating, created, nil
}

func scanPipeline(p *pipeline, scan scannable) error {
	var (
		groups      sql.NullString
		varSources  sql.NullString
		nonce       sql.NullString
		nonceStr    *string
		lastUpdated pq.NullTime
	)
	err := scan.Scan(&p.id, &p.name, &groups, &varSources, &nonce, &p.configVersion, &p.teamID, &p.teamName, &p.paused, &p.public, &p.archived, &lastUpdated)
	if err != nil {
		return err
	}

	p.lastUpdated = lastUpdated.Time

	if groups.Valid {
		var pipelineGroups atc.GroupConfigs
		err = json.Unmarshal([]byte(groups.String), &pipelineGroups)
		if err != nil {
			return err
		}

		p.groups = pipelineGroups
	}

	if nonce.Valid {
		nonceStr = &nonce.String
	}

	if varSources.Valid {
		var pipelineVarSources atc.VarSourceConfigs
		decryptedVarSource, err := p.conn.EncryptionStrategy().Decrypt(varSources.String, nonceStr)
		if err != nil {
			return err
		}
		err = json.Unmarshal([]byte(decryptedVarSource), &pipelineVarSources)
		if err != nil {
			return err
		}

		p.varSources = pipelineVarSources
	}

	return nil
}

func scanPipelines(conn Conn, lockFactory lock.LockFactory, rows *sql.Rows) ([]Pipeline, error) {
	defer Close(rows)

	pipelines := []Pipeline{}

	for rows.Next() {
		pipeline := newPipeline(conn, lockFactory)

		err := scanPipeline(pipeline, rows)
		if err != nil {
			return nil, err
		}

		pipelines = append(pipelines, pipeline)
	}

	return pipelines, nil
}

func scanContainers(rows *sql.Rows, conn Conn, initContainers []Container) ([]Container, error) {
	containers := initContainers

	defer Close(rows)

	for rows.Next() {
		creating, created, destroying, _, err := scanContainer(rows, conn)
		if err != nil {
			return []Container{}, err
		}

		if creating != nil {
			containers = append(containers, creating)
		}

		if created != nil {
			containers = append(containers, created)
		}

		if destroying != nil {
			containers = append(containers, destroying)
		}
	}

	return containers, nil
}

func (t *team) queryTeam(tx Tx, query string, params ...interface{}) error {
	var providerAuth, nonce sql.NullString

	err := tx.QueryRow(query, params...).Scan(
		&t.id,
		&t.name,
		&t.admin,
		&providerAuth,
		&nonce,
	)
	if err != nil {
		return err
	}

	if providerAuth.Valid {
		var auth atc.TeamAuth
		err = json.Unmarshal([]byte(providerAuth.String), &auth)
		if err != nil {
			return err
		}
		t.auth = auth
	}

	return nil
}

func (t *team) resetDependentTableStates(tx Tx, pipelineID int) error {
	_, err := psql.Delete("jobs_serial_groups").
		Where(sq.Expr(`job_id in (
        SELECT j.id
        FROM jobs j
        WHERE j.pipeline_id = $1
      )`, pipelineID)).
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	tableNames := []string{"jobs", "resources", "resource_types"}
	for _, table := range tableNames {
		err = t.inactivateTableForPipeline(tx, pipelineID, table)
		if err != nil {
			return err
		}
	}
	return err
}

func (t *team) inactivateTableForPipeline(tx Tx, pipelineID int, tableName string) error {
	_, err := psql.Update(tableName).
		Set("active", false).
		Where(sq.Eq{
			"pipeline_id": pipelineID,
		}).
		RunWith(tx).
		Exec()
	return err
}

func (t *team) saveResources(tx Tx, resources atc.ResourceConfigs, pipelineID int) (map[string]int, error) {
	resourceNameToID := make(map[string]int)
	for _, resource := range resources {
		resourceID, err := t.saveResource(tx, resource, pipelineID)
		if err != nil {
			return nil, err
		}

		resourceNameToID[resource.Name] = resourceID
	}

	return resourceNameToID, nil
}

func (t *team) saveResourceTypes(tx Tx, resourceTypes atc.ResourceTypes, pipelineID int) error {
	for _, resourceType := range resourceTypes {
		err := t.saveResourceType(tx, resourceType, pipelineID)
		if err != nil {
			return err
		}
	}

	return nil
}

func (t *team) saveJobsAndSerialGroups(tx Tx, jobs atc.JobConfigs, groups atc.GroupConfigs, pipelineID int) (map[string]int, error) {
	jobGroups := make(map[string][]string)
	for _, group := range groups {
		for _, job := range group.Jobs {
			jobGroups[job] = append(jobGroups[job], group.Name)
		}
	}

	jobNameToID := make(map[string]int)
	for _, job := range jobs {
		jobID, err := t.saveJob(tx, job, pipelineID, jobGroups[job.Name])
		if err != nil {
			return nil, err
		}

		jobNameToID[job.Name] = jobID

		if len(job.SerialGroups) != 0 {
			for _, sg := range job.SerialGroups {
				err = t.registerSerialGroup(tx, sg, jobID)
				if err != nil {
					return nil, err
				}
			}
		} else {
			if job.Serial || job.RawMaxInFlight > 0 {
				err = t.registerSerialGroup(tx, job.Name, jobID)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	return jobNameToID, nil
}

func (t *team) insertJobPipes(tx Tx, jobConfigs atc.JobConfigs, resourceNameToID map[string]int, jobNameToID map[string]int, pipelineID int) error {
	_, err := psql.Delete("job_inputs").
		Where(sq.Expr(`job_id in (
        SELECT j.id
        FROM jobs j
        WHERE j.pipeline_id = $1
      )`, pipelineID)).
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	_, err = psql.Delete("job_outputs").
		Where(sq.Expr(`job_id in (
        SELECT j.id
        FROM jobs j
        WHERE j.pipeline_id = $1
      )`, pipelineID)).
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	for _, jobConfig := range jobConfigs {
		for _, plan := range jobConfig.Plans() {
			if plan.Get != "" {
				err = insertJobInput(tx, plan, jobConfig.Name, resourceNameToID, jobNameToID)
				if err != nil {
					return err
				}
			} else if plan.Put != "" {
				err = insertJobOutput(tx, plan, jobConfig.Name, resourceNameToID, jobNameToID)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func insertJobInput(tx Tx, plan atc.PlanConfig, jobName string, resourceNameToID map[string]int, jobNameToID map[string]int) error {
	if len(plan.Passed) != 0 {
		for _, passedJob := range plan.Passed {
			var resourceID int
			if plan.Resource != "" {
				resourceID = resourceNameToID[plan.Resource]
			} else {
				resourceID = resourceNameToID[plan.Get]
			}

			var version sql.NullString
			if plan.Version != nil {
				versionJSON, err := plan.Version.MarshalJSON()
				if err != nil {
					return err
				}

				version = sql.NullString{Valid: true, String: string(versionJSON)}
			}

			_, err := psql.Insert("job_inputs").
				Columns("name", "job_id", "resource_id", "passed_job_id", "trigger", "version").
				Values(plan.Get, jobNameToID[jobName], resourceID, jobNameToID[passedJob], plan.Trigger, version).
				RunWith(tx).
				Exec()
			if err != nil {
				return err
			}
		}
	} else {
		var resourceID int
		if plan.Resource != "" {
			resourceID = resourceNameToID[plan.Resource]
		} else {
			resourceID = resourceNameToID[plan.Get]
		}

		var version sql.NullString
		if plan.Version != nil {
			versionJSON, err := plan.Version.MarshalJSON()
			if err != nil {
				return err
			}

			version = sql.NullString{Valid: true, String: string(versionJSON)}
		}

		_, err := psql.Insert("job_inputs").
			Columns("name", "job_id", "resource_id", "trigger", "version").
			Values(plan.Get, jobNameToID[jobName], resourceID, plan.Trigger, version).
			RunWith(tx).
			Exec()
		if err != nil {
			return err
		}
	}

	return nil
}

func insertJobOutput(tx Tx, plan atc.PlanConfig, jobName string, resourceNameToID map[string]int, jobNameToID map[string]int) error {
	var resourceID int
	if plan.Resource != "" {
		resourceID = resourceNameToID[plan.Resource]
	} else {
		resourceID = resourceNameToID[plan.Put]
	}

	_, err := psql.Insert("job_outputs").
		Columns("name", "job_id", "resource_id").
		Values(plan.Put, jobNameToID[jobName], resourceID).
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	return nil
}
