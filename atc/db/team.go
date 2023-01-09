package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"

	sq "github.com/Masterminds/squirrel"
	"github.com/gobwas/glob"
	"github.com/lib/pq"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/encryption"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/event"
)

var ErrConfigComparisonFailed = errors.New("comparison with existing config failed during save")

type ErrPipelineNotFound atc.PipelineRef

func (e ErrPipelineNotFound) Error() string {
	return fmt.Sprintf("pipeline '%s' not found", atc.PipelineRef(e))
}

//counterfeiter:generate . Team
type Team interface {
	ID() int
	Name() string
	Admin() bool

	Auth() atc.TeamAuth

	Delete() error
	Rename(string) error

	SavePipeline(
		pipelineRef atc.PipelineRef,
		config atc.Config,
		from ConfigVersion,
		initiallyPaused bool,
	) (Pipeline, bool, error)
	RenamePipeline(oldName string, newName string) (bool, error)

	Pipeline(pipelineRef atc.PipelineRef) (Pipeline, bool, error)
	Pipelines() ([]Pipeline, error)
	PublicPipelines() ([]Pipeline, error)
	OrderPipelines([]string) error
	OrderPipelinesWithinGroup(string, []atc.InstanceVars) error

	CreateOneOffBuild() (Build, error)
	CreateStartedBuild(plan atc.Plan) (Build, error)

	PrivateAndPublicBuilds(Page) ([]BuildForAPI, Pagination, error)
	Builds(page Page) ([]BuildForAPI, Pagination, error)
	BuildsWithTime(page Page) ([]BuildForAPI, Pagination, error)

	SaveWorker(atcWorker atc.Worker, ttl time.Duration) (Worker, error)
	Workers() ([]Worker, error)
	FindVolumeForWorkerArtifact(int) (CreatedVolume, bool, error)

	Containers() ([]Container, error)
	IsCheckContainer(string) (bool, error)
	IsContainerWithinTeam(string, bool) (bool, error)

	FindContainerByHandle(string) (Container, bool, error)
	FindCheckContainers(lager.Logger, atc.PipelineRef, string) ([]Container, map[int]time.Time, error)
	FindContainersByMetadata(ContainerMetadata) ([]Container, error)
	FindCreatedContainerByHandle(string) (CreatedContainer, bool, error)
	FindWorkerForContainer(handle string) (Worker, bool, error)
	FindWorkerForVolume(handle string) (Worker, bool, error)
	FindWorkersForResourceCache(rcId int, shouldBeValidBefore time.Time) ([]Worker, error)

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

// FindWorkersForResourceCache returns workers that contain valid resource caches.
// A valid resource cache's worker_base_resource_type_id is not nil. If an invalidated
// worker resource cache is only got invalidated after shouldBeValidBefore, then it can
// also be returned, where shouldBeValidBefore is usually a build start time, meaning
// that, if a worker resource cache is invalidated after a build is started, then the
// build can still use the cache.
func (t *team) FindWorkersForResourceCache(rcId int, shouldBeValidBefore time.Time) ([]Worker, error) {
	return getWorkers(
		t.conn, workersQuery.
			Join("worker_resource_caches wrc ON w.name = wrc.worker_name").
			Where(sq.And{
				sq.Eq{"wrc.resource_cache_id": rcId},
				sq.Or{
					sq.NotEq{"wrc.worker_base_resource_type_id": nil},
					sq.Expr("wrc.invalid_since > to_timestamp(?)", shouldBeValidBefore.Unix()),
				},
				sq.Eq{"w.state": WorkerStateRunning},
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
		Join("workers w ON c.worker_name = w.name").
		Join("resource_config_check_sessions rccs ON rccs.id = c.resource_config_check_session_id").
		Join("prototypes pt ON pt.resource_config_id = rccs.resource_config_id").
		Join("pipelines p ON p.id = pt.pipeline_id").
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
	var ok int
	err := psql.Select("1").
		From("containers").
		Where(sq.Eq{
			"handle": handle,
		}).
		Where(sq.NotEq{
			"resource_config_check_session_id": nil,
		}).
		RunWith(t.conn).
		QueryRow().
		Scan(&ok)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}

		return false, err
	}

	return true, nil
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

func savePipeline(
	tx Tx,
	pipelineRef atc.PipelineRef,
	config atc.Config,
	from ConfigVersion,
	initiallyPaused bool,
	teamID int,
	jobID sql.NullInt64,
	buildID sql.NullInt64,
) (int, bool, error) {

	var instanceVars sql.NullString
	if pipelineRef.InstanceVars != nil {
		bytes, _ := json.Marshal(pipelineRef.InstanceVars)
		instanceVars = sql.NullString{
			String: string(bytes),
			Valid:  true,
		}
	}

	pipelineRefWhereClause := sq.Eq{
		"team_id":       teamID,
		"name":          pipelineRef.Name,
		"instance_vars": instanceVars,
	}

	var existingConfig bool
	err := psql.Select("1").
		From("pipelines").
		Where(pipelineRefWhereClause).
		Prefix("SELECT EXISTS (").Suffix(")").
		RunWith(tx).
		QueryRow().
		Scan(&existingConfig)
	if err != nil {
		return 0, false, err
	}

	groupsPayload, err := json.Marshal(config.Groups)
	if err != nil {
		return 0, false, err
	}

	varSourcesPayload, err := json.Marshal(config.VarSources)
	if err != nil {
		return 0, false, err
	}

	encryptedVarSourcesPayload, nonce, err := tx.EncryptionStrategy().Encrypt(varSourcesPayload)
	if err != nil {
		return 0, false, err
	}

	displayPayload, err := json.Marshal(config.Display)
	if err != nil {
		return 0, false, err
	}

	var pipelineID int
	if !existingConfig {
		values := map[string]interface{}{
			"name":            pipelineRef.Name,
			"groups":          groupsPayload,
			"var_sources":     encryptedVarSourcesPayload,
			"display":         displayPayload,
			"nonce":           nonce,
			"version":         sq.Expr("nextval('config_version_seq')"),
			"paused":          initiallyPaused,
			"last_updated":    sq.Expr("now()"),
			"team_id":         teamID,
			"parent_job_id":   jobID,
			"parent_build_id": buildID,
			"instance_vars":   instanceVars,
		}
		var ordering sql.NullInt64
		var secondaryOrdering sql.NullInt64
		err := psql.Select("max(ordering), max(secondary_ordering)").
			From("pipelines").
			Where(sq.Eq{
				"team_id": teamID,
				"name":    pipelineRef.Name,
			}).
			RunWith(tx).
			QueryRow().
			Scan(&ordering, &secondaryOrdering)
		if err != nil {
			return 0, false, err
		}
		if ordering.Valid {
			values["ordering"] = ordering.Int64
			values["secondary_ordering"] = secondaryOrdering.Int64 + 1
		} else {
			values["ordering"] = sq.Expr("currval('pipelines_id_seq')")
			values["secondary_ordering"] = 1
		}
		err = psql.Insert("pipelines").
			SetMap(values).
			Suffix("RETURNING id").
			RunWith(tx).
			QueryRow().Scan(&pipelineID)
		if err != nil {
			return 0, false, err
		}

	} else {
		q := psql.Update("pipelines").
			Set("archived", false).
			Set("groups", groupsPayload).
			Set("var_sources", encryptedVarSourcesPayload).
			Set("display", displayPayload).
			Set("nonce", nonce).
			Set("version", sq.Expr("nextval('config_version_seq')")).
			Set("last_updated", sq.Expr("now()")).
			Set("parent_job_id", jobID).
			Set("parent_build_id", buildID).
			Where(sq.And{
				pipelineRefWhereClause,
				sq.Eq{"version": from},
			})

		if !initiallyPaused {
			// The set_pipeline step creates pipelines that aren't initially
			// paused (unlike `fly set-pipeline`). In this case, we should keep
			// previously paused pipelines as paused, but we should unpause any
			// formerly archived pipelines.
			q = q.Set("paused", sq.Expr("paused AND NOT archived"))
		}
		if buildID.Valid {
			q = q.Where(sq.Or{sq.Lt{"parent_build_id": buildID}, sq.Eq{"parent_build_id": nil}})
		}

		err := q.Suffix("RETURNING id").
			RunWith(tx).
			QueryRow().
			Scan(&pipelineID)
		if err != nil {
			if err == sql.ErrNoRows {
				var currentParentBuildID sql.NullInt64
				err := psql.Select("parent_build_id").
					From("pipelines").
					Where(pipelineRefWhereClause).
					RunWith(tx).
					QueryRow().
					Scan(&currentParentBuildID)
				if err != nil {
					return 0, false, err
				}
				if currentParentBuildID.Valid && int(buildID.Int64) < int(currentParentBuildID.Int64) {
					return 0, false, ErrSetByNewerBuild
				}
				return 0, false, ErrConfigComparisonFailed
			}

			return 0, false, err
		}

		err = resetDependentTableStates(tx, pipelineID)
		if err != nil {
			return 0, false, err
		}
	}

	err = updateResourcesName(tx, config.Resources, pipelineID)
	if err != nil {
		return 0, false, err
	}

	resourceNameToID, err := saveResources(tx, config.Resources, pipelineID)
	if err != nil {
		return 0, false, err
	}

	err = saveResourceTypes(tx, config.ResourceTypes, pipelineID)
	if err != nil {
		return 0, false, err
	}

	err = savePrototypes(tx, config.Prototypes, pipelineID)
	if err != nil {
		return 0, false, err
	}

	err = updateJobsName(tx, config.Jobs, pipelineID)
	if err != nil {
		return 0, false, err
	}

	jobNameToID, err := saveJobsAndSerialGroups(tx, config.Jobs, config.Groups, pipelineID)
	if err != nil {
		return 0, false, err
	}

	err = removeUnusedWorkerTaskCaches(tx, pipelineID, config.Jobs)
	if err != nil {
		return 0, false, err
	}

	err = insertJobPipes(tx, config.Jobs, resourceNameToID, jobNameToID, pipelineID)
	if err != nil {
		return 0, false, err
	}

	err = requestScheduleForJobsInPipeline(tx, pipelineID)
	if err != nil {
		return 0, false, err
	}

	return pipelineID, !existingConfig, nil
}

func (t *team) SavePipeline(
	pipelineRef atc.PipelineRef,
	config atc.Config,
	from ConfigVersion,
	initiallyPaused bool,
) (Pipeline, bool, error) {
	tx, err := t.conn.Begin()
	if err != nil {
		return nil, false, err
	}

	defer Rollback(tx)

	nullID := sql.NullInt64{Valid: false}
	pipelineID, isNewPipeline, err := savePipeline(tx, pipelineRef, config, from, initiallyPaused, t.id, nullID, nullID)
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

	err = tx.Commit()
	if err != nil {
		return nil, false, err
	}

	return pipeline, isNewPipeline, nil
}

func (t *team) RenamePipeline(oldName, newName string) (bool, error) {
	result, err := psql.Update("pipelines").
		Set("name", newName).
		Where(sq.Eq{
			"team_id": t.id,
			"name":    oldName,
		}).
		RunWith(t.conn).
		Exec()
	if err != nil {
		return false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}

func (t *team) Pipeline(pipelineRef atc.PipelineRef) (Pipeline, bool, error) {
	pipeline := newPipeline(t.conn, t.lockFactory)

	var instanceVars sql.NullString
	if pipelineRef.InstanceVars != nil {
		bytes, _ := json.Marshal(pipelineRef.InstanceVars)
		instanceVars = sql.NullString{
			String: string(bytes),
			Valid:  true,
		}
	}

	err := scanPipeline(
		pipeline,
		pipelinesQuery.
			Where(sq.Eq{
				"p.team_id":       t.id,
				"p.name":          pipelineRef.Name,
				"p.instance_vars": instanceVars,
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
		OrderBy("p.ordering", "p.secondary_ordering").
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
		OrderBy("p.ordering ASC", "p.secondary_ordering ASC").
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

func (t *team) OrderPipelines(names []string) error {
	tx, err := t.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	for i, name := range names {
		pipelineUpdate, err := psql.Update("pipelines").
			Set("ordering", i).
			Where(sq.Eq{
				"team_id": t.id,
				"name":    name,
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
			return ErrPipelineNotFound{Name: name}
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (t *team) OrderPipelinesWithinGroup(groupName string, instanceVars []atc.InstanceVars) error {

	tx, err := t.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	for i, vars := range instanceVars {
		filter := sq.Eq{
			"team_id": t.id,
			"name":    groupName,
		}

		if len(vars) == 0 {
			filter["instance_vars"] = nil
		} else {
			varsJson, err := json.Marshal(vars)
			if err != nil {
				return err
			}
			filter["instance_vars"] = varsJson
		}

		pipelineUpdate, err := psql.Update("pipelines").
			Set("secondary_ordering", i+1).
			Where(filter).
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
			return ErrPipelineNotFound{Name: groupName, InstanceVars: vars}
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
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

func (t *team) PrivateAndPublicBuilds(page Page) ([]BuildForAPI, Pagination, error) {
	newBuildsQuery := buildsQuery.
		Where(sq.Or{sq.Eq{"p.public": true}, sq.Eq{"t.id": t.id}})

	return getBuildsWithPagination(newBuildsQuery, minMaxIdQuery, page, t.conn, t.lockFactory, false)
}

func (t *team) BuildsWithTime(page Page) ([]BuildForAPI, Pagination, error) {
	return getBuildsWithDates(buildsQuery.Where(sq.Eq{"t.id": t.id}), minMaxIdQuery, page, t.conn, t.lockFactory)
}

func (t *team) Builds(page Page) ([]BuildForAPI, Pagination, error) {
	return getBuildsWithPagination(buildsQuery.Where(sq.Eq{"t.id": t.id}), minMaxIdQuery, page, t.conn, t.lockFactory, false)
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

func (t *team) FindCheckContainers(logger lager.Logger, pipelineRef atc.PipelineRef, resourceName string) ([]Container, map[int]time.Time, error) {
	pipeline, found, err := t.Pipeline(pipelineRef)
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

	rows, err := selectContainers("c").
		Join("resource_config_check_sessions rccs ON rccs.id = c.resource_config_check_session_id").
		Where(sq.Eq{
			"rccs.resource_config_id": resource.ResourceConfigID(),
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
			"rccs.resource_config_id": resource.ResourceConfigID(),
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

func updateJobsName(tx Tx, jobs []atc.JobConfig, pipelineID int) error {
	jobsToUpdate := []UpdateName{}

	for _, job := range jobs {
		if job.OldName != "" && job.OldName != job.Name {
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

func updateResourcesName(tx Tx, resources []atc.ResourceConfig, pipelineID int) error {
	resourcesToUpdate := []UpdateName{}

	for _, res := range resources {
		if res.OldName != "" && res.OldName != res.Name {
			var count int
			err := psql.Select("COUNT(*) as count").
				From("resources").
				Where(sq.Eq{
					"name":        res.OldName,
					"pipeline_id": pipelineID}).
				RunWith(tx).
				QueryRow().
				Scan(&count)
			if err != nil {
				return err
			}

			if count != 0 {
				resourcesToUpdate = append(resourcesToUpdate, UpdateName{
					OldName: res.OldName,
					NewName: res.Name,
				})
			}
		}
	}

	newMap := make(map[int]bool)
	for _, updateNames := range resourcesToUpdate {
		isCyclic := checkCyclic(resourcesToUpdate, updateNames.OldName, newMap)
		if isCyclic {
			return errors.New("resource name swapping is not supported at this time")
		}
	}

	resourcesToUpdate = sortUpdateNames(resourcesToUpdate)

	for _, updateName := range resourcesToUpdate {
		_, err := psql.Delete("resources").
			Where(sq.Eq{
				"name":        updateName.NewName,
				"pipeline_id": pipelineID}).
			RunWith(tx).
			Exec()
		if err != nil {
			return err
		}

		_, err = psql.Update("resources").
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

func checkCyclic(updateNames []UpdateName, curr string, visited map[int]bool) bool {
	for i, updateName := range updateNames {
		if updateName.NewName == curr && !visited[i] {
			visited[i] = true
			checkCyclic(updateNames, updateName.OldName, visited)
		} else if updateName.NewName == curr && visited[i] && curr != updateName.OldName {
			return true
		}
	}

	return false
}

func sortUpdateNames(updateNames []UpdateName) []UpdateName {
	newMap := make(map[string]int)
	for i, updateName := range updateNames {
		newMap[updateName.NewName] = i + 1

		if newMap[updateName.OldName] != 0 {
			index := newMap[updateName.OldName] - 1

			tempName := updateNames[index]
			updateNames[index] = updateName
			updateNames[i] = tempName

			return sortUpdateNames(updateNames)
		}
	}

	return updateNames
}

func saveJob(tx Tx, job atc.JobConfig, pipelineID int, groups []string) (int, error) {
	configPayload, err := json.Marshal(job)
	if err != nil {
		return 0, err
	}

	es := tx.EncryptionStrategy()
	encryptedPayload, nonce, err := es.Encrypt(configPayload)
	if err != nil {
		return 0, err
	}

	var jobID int
	err = psql.Insert("jobs").
		Columns("name", "pipeline_id", "config", "public", "max_in_flight", "disable_manual_trigger", "interruptible", "active", "nonce", "tags").
		Values(job.Name, pipelineID, encryptedPayload, job.Public, job.MaxInFlight(), job.DisableManualTrigger, job.Interruptible, true, nonce, pq.Array(groups)).
		Suffix("ON CONFLICT (name, pipeline_id) DO UPDATE SET config = EXCLUDED.config, public = EXCLUDED.public, max_in_flight = EXCLUDED.max_in_flight, disable_manual_trigger = EXCLUDED.disable_manual_trigger, interruptible = EXCLUDED.interruptible, active = EXCLUDED.active, nonce = EXCLUDED.nonce, tags = EXCLUDED.tags").
		Suffix("RETURNING id").
		RunWith(tx).
		QueryRow().
		Scan(&jobID)
	if err != nil {
		return 0, err
	}

	return jobID, nil
}

func registerSerialGroup(tx Tx, serialGroup string, jobID int) error {
	_, err := psql.Insert("jobs_serial_groups").
		Columns("serial_group", "job_id").
		Values(serialGroup, jobID).
		RunWith(tx).
		Exec()
	return err
}

func saveResourceType(tx Tx, resourceType atc.ResourceType, pipelineID int) error {
	configPayload, err := json.Marshal(resourceType)
	if err != nil {
		return err
	}

	es := tx.EncryptionStrategy()
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

func savePrototype(tx Tx, prototype atc.Prototype, pipelineID int) error {
	configPayload, err := json.Marshal(prototype)
	if err != nil {
		return err
	}

	es := tx.EncryptionStrategy()
	encryptedPayload, nonce, err := es.Encrypt(configPayload)
	if err != nil {
		return err
	}

	_, err = psql.Insert("prototypes").
		Columns("name", "pipeline_id", "config", "active", "nonce", "type").
		Values(prototype.Name, pipelineID, encryptedPayload, true, nonce, prototype.Type).
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
		groups        sql.NullString
		varSources    sql.NullString
		display       sql.NullString
		nonce         sql.NullString
		nonceStr      *string
		lastUpdated   pq.NullTime
		parentJobID   sql.NullInt64
		parentBuildID sql.NullInt64
		instanceVars  sql.NullString
		pausedBy      sql.NullString
		pausedAt      sql.NullTime
	)
	err := scan.Scan(&p.id, &p.name, &groups, &varSources, &display, &nonce, &p.configVersion, &p.teamID, &p.teamName, &p.paused, &p.public, &p.archived, &lastUpdated, &parentJobID, &parentBuildID, &instanceVars, &pausedBy, &pausedAt)
	if err != nil {
		return err
	}

	p.lastUpdated = lastUpdated.Time
	p.parentJobID = int(parentJobID.Int64)
	p.parentBuildID = int(parentBuildID.Int64)

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

	if display.Valid {
		var displayConfig *atc.DisplayConfig
		err = json.Unmarshal([]byte(display.String), &displayConfig)
		if err != nil {
			return err
		}

		p.display = displayConfig
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

	if instanceVars.Valid {
		err = json.Unmarshal([]byte(instanceVars.String), &p.instanceVars)
		if err != nil {
			return err
		}
	}

	if pausedBy.Valid {
		p.pausedBy = pausedBy.String
	}

	if pausedAt.Valid {
		p.pausedAt = pausedAt.Time
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

func resetDependentTableStates(tx Tx, pipelineID int) error {
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

	for _, table := range pipelineObjectTables {
		err = inactivateTableForPipeline(tx, pipelineID, table)
		if err != nil {
			return err
		}
	}
	return err
}

func inactivateTableForPipeline(tx Tx, pipelineID int, tableName string) error {
	_, err := psql.Update(tableName).
		Set("active", false).
		Where(sq.Eq{
			"pipeline_id": pipelineID,
		}).
		RunWith(tx).
		Exec()
	return err
}

func saveResources(tx Tx, resources atc.ResourceConfigs, pipelineID int) (map[string]int, error) {
	if len(resources) == 0 {
		return nil, nil
	}
	insertQuery := psql.Insert("resources").
		Columns("name", "pipeline_id", "config", "active", "nonce", "type", "resource_config_id", "resource_config_scope_id").
		Suffix("ON CONFLICT (name, pipeline_id) DO UPDATE SET config = EXCLUDED.config, active = EXCLUDED.active, nonce = EXCLUDED.nonce, type = EXCLUDED.type, resource_config_id = EXCLUDED.resource_config_id, resource_config_scope_id = EXCLUDED.resource_config_scope_id").
		Suffix("RETURNING name, id")
	resourcesToPin := map[string][]byte{}

	existingResources, err := existingResources(tx, pipelineID)
	if err != nil {
		return nil, nil
	}

	for _, resource := range resources {
		configPayload, err := json.Marshal(resource)
		if err != nil {
			return nil, err
		}

		es := tx.EncryptionStrategy()
		encryptedPayload, nonce, err := es.Encrypt(configPayload)
		if err != nil {
			return nil, err
		}
		values := []interface{}{resource.Name, pipelineID, encryptedPayload, true, nonce, resource.Type}

		existing, exists := existingResources[resource.Name]

		if !exists {
			values = append(values, nil, nil)
		} else {
			configsDiffer, err := configsDifferent(resource, encryptedPayload, existing, es)
			if err != nil {
				return nil, err
			}
			if configsDiffer || resource.Type != existing.Type {
				values = append(values, nil, nil)
			} else {
				values = append(values, existing.ResourceConfigID, existing.ResourceConfigScopeID)
			}
		}

		if resource.Version != nil {
			version, err := json.Marshal(resource.Version)
			if err != nil {
				return nil, err
			}
			resourcesToPin[resource.Name] = version
		}

		insertQuery = insertQuery.Values(values...)
	}

	resourceNameToID := make(map[string]int)
	resourceIDs := []int{}

	rows, err := insertQuery.RunWith(tx).Query()
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var (
			name string
			id   int
		)
		err = rows.Scan(&name, &id)
		if err != nil {
			return nil, err
		}
		resourceNameToID[name] = id
		resourceIDs = append(resourceIDs, id)
	}
	err = resetResourcePins(tx, resourceIDs, resourcesToPin, resourceNameToID)
	if err != nil {
		return nil, err
	}
	return resourceNameToID, nil
}

type existingResource struct {
	Type                  string
	ConfigBlob            string
	Nonce                 *string
	ResourceConfigID      interface{}
	ResourceConfigScopeID interface{}
}

func existingResources(tx Tx, pipelineID int) (map[string]existingResource, error) {
	rows, err := psql.Select("name", "config", "type", "nonce", "resource_config_id", "resource_config_scope_id").
		From("resources").
		Where(sq.Eq{"pipeline_id": pipelineID}).
		RunWith(tx).
		Query()
	if err != nil {
		return nil, err
	}
	defer Close(rows)

	resources := map[string]existingResource{}

	for rows.Next() {
		var configBlob, nonce sql.NullString
		var rcID, rcScopeID sql.NullInt64
		var name string
		r := &existingResource{}
		err = rows.Scan(&name, &configBlob, &r.Type, &nonce, &rcID, &rcScopeID)
		if err != nil {
			return nil, err
		}

		if nonce.Valid {
			r.Nonce = &nonce.String
		}

		if configBlob.Valid {
			r.ConfigBlob = configBlob.String
		}

		r.ResourceConfigID, _ = rcID.Value()
		r.ResourceConfigScopeID, _ = rcScopeID.Value()
		resources[name] = *r
	}

	return resources, nil
}

func configsDifferent(resourceConfig atc.ResourceConfig, encryptedBlob string, existing existingResource, es encryption.Strategy) (bool, error) {
	if encryptedBlob == existing.ConfigBlob {
		return false, nil
	}

	// Archived pipelines may have a NULL existing config
	if existing.ConfigBlob == "" {
		return true, nil
	}

	decryptedConfig, err := es.Decrypt(existing.ConfigBlob, existing.Nonce)
	if err != nil {
		return false, err
	}

	existingConfig := atc.ResourceConfig{}
	if string(decryptedConfig) != "" {
		err = json.Unmarshal(decryptedConfig, &existingConfig)
		if err != nil {
			return false, err
		}
	}

	return mapHash(resourceConfig.Source) != mapHash(existingConfig.Source), nil
}

func resetResourcePins(tx Tx, resourceIDs []int, resourcesToPin map[string][]byte, resourceNameToID map[string]int) error {
	ids := []string{}
	for _, resourceID := range resourceIDs {
		ids = append(ids, strconv.Itoa(resourceID))
	}
	_, err := psql.Delete("resource_pins").
		Where(sq.Eq{"config": true}).
		Where(sq.Expr(`resource_id IN (` + strings.Join(ids, ",") + `)`)).
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}
	if len(resourcesToPin) > 0 {
		pinQuery := psql.Insert("resource_pins").
			Columns("resource_id", "version", "comment_text", "config").
			Suffix("ON CONFLICT (resource_id) DO UPDATE SET version = EXCLUDED.version, comment_text = EXCLUDED.comment_text, config = true")

		for resource, version := range resourcesToPin {
			pinQuery = pinQuery.Values(resourceNameToID[resource], version, "", true)
		}

		_, err = pinQuery.RunWith(tx).Exec()
		return err
	}
	return nil
}

func saveResourceTypes(tx Tx, resourceTypes atc.ResourceTypes, pipelineID int) error {
	for _, resourceType := range resourceTypes {
		err := saveResourceType(tx, resourceType, pipelineID)
		if err != nil {
			return err
		}
	}

	return nil
}

func savePrototypes(tx Tx, prototypes atc.Prototypes, pipelineID int) error {
	for _, prototype := range prototypes {
		err := savePrototype(tx, prototype, pipelineID)
		if err != nil {
			return err
		}
	}

	return nil
}

func saveJobsAndSerialGroups(tx Tx, jobs atc.JobConfigs, groups atc.GroupConfigs, pipelineID int) (map[string]int, error) {
	jobGroups := make(map[string][]string)
	for _, group := range groups {
		for _, jobGlob := range group.Jobs {
			for _, job := range jobs {
				if g, err := glob.Compile(jobGlob); err == nil && g.Match(job.Name) {
					jobGroups[job.Name] = append(jobGroups[job.Name], group.Name)
				}
			}
		}
	}

	jobNameToID := make(map[string]int)
	for _, job := range jobs {
		jobID, err := saveJob(tx, job, pipelineID, jobGroups[job.Name])
		if err != nil {
			return nil, err
		}

		jobNameToID[job.Name] = jobID

		if len(job.SerialGroups) != 0 {
			for _, sg := range job.SerialGroups {
				err = registerSerialGroup(tx, sg, jobID)
				if err != nil {
					return nil, err
				}
			}
		} else {
			if job.Serial || job.RawMaxInFlight > 0 {
				err = registerSerialGroup(tx, job.Name, jobID)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	return jobNameToID, nil
}

func insertJobPipes(tx Tx, jobConfigs atc.JobConfigs, resourceNameToID map[string]int, jobNameToID map[string]int, pipelineID int) error {
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
		err := jobConfig.StepConfig().Visit(atc.StepRecursor{
			OnGet: func(step *atc.GetStep) error {
				return insertJobInput(tx, step, jobConfig.Name, resourceNameToID, jobNameToID)
			},
			OnPut: func(step *atc.PutStep) error {
				return insertJobOutput(tx, step, jobConfig.Name, resourceNameToID, jobNameToID)
			},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func insertJobInput(tx Tx, step *atc.GetStep, jobName string, resourceNameToID map[string]int, jobNameToID map[string]int) error {
	if len(step.Passed) != 0 {
		for _, passedJob := range step.Passed {
			var version sql.NullString
			if step.Version != nil {
				versionJSON, err := step.Version.MarshalJSON()
				if err != nil {
					return err
				}

				version = sql.NullString{Valid: true, String: string(versionJSON)}
			}

			_, err := psql.Insert("job_inputs").
				Columns("name", "job_id", "resource_id", "passed_job_id", "trigger", "version").
				Values(step.Name, jobNameToID[jobName], resourceNameToID[step.ResourceName()], jobNameToID[passedJob], step.Trigger, version).
				RunWith(tx).
				Exec()
			if err != nil {
				return err
			}
		}
	} else {
		var version sql.NullString
		if step.Version != nil {
			versionJSON, err := step.Version.MarshalJSON()
			if err != nil {
				return err
			}

			version = sql.NullString{Valid: true, String: string(versionJSON)}
		}

		_, err := psql.Insert("job_inputs").
			Columns("name", "job_id", "resource_id", "trigger", "version").
			Values(step.Name, jobNameToID[jobName], resourceNameToID[step.ResourceName()], step.Trigger, version).
			RunWith(tx).
			Exec()
		if err != nil {
			return err
		}
	}

	return nil
}

func insertJobOutput(tx Tx, step *atc.PutStep, jobName string, resourceNameToID map[string]int, jobNameToID map[string]int) error {
	_, err := psql.Insert("job_outputs").
		Columns("name", "job_id", "resource_id").
		Values(step.Name, jobNameToID[jobName], resourceNameToID[step.ResourceName()]).
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	return nil
}
