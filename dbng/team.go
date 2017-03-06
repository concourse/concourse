package dbng

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db/lock"
	"github.com/lib/pq"
	uuid "github.com/nu7hatch/gouuid"
)

var ErrConfigComparisonFailed = errors.New("comparison-with-existing-config-failed-during-save")
var ErrTeamDisappeared = errors.New("team-disappeared")

//go:generate counterfeiter . Team

type Team interface {
	ID() int
	SavePipeline(
		pipelineName string,
		config atc.Config,
		from ConfigVersion,
		pausedState PipelinePausedState,
	) (Pipeline, bool, error)

	FindPipelineByName(pipelineName string) (Pipeline, bool, error)

	CreateOneOffBuild() (Build, error)

	SaveWorker(atcWorker atc.Worker, ttl time.Duration) (Worker, error)
	Workers() ([]Worker, error)

	FindContainerByHandle(string) (CreatedContainer, bool, error)

	FindResourceCheckContainer(workerName string, resourceConfig *UsedResourceConfig) (CreatingContainer, CreatedContainer, error)
	CreateResourceCheckContainer(workerName string, resourceConfig *UsedResourceConfig) (CreatingContainer, error)

	CreateResourceGetContainer(workerName string, resourceConfig *UsedResourceCache, stepName string) (CreatingContainer, error)

	FindBuildContainer(workerName string, buildID int, planID atc.PlanID, meta ContainerMetadata) (CreatingContainer, CreatedContainer, error)
	CreateBuildContainer(workerName string, buildID int, planID atc.PlanID, meta ContainerMetadata) (CreatingContainer, error)
}

type team struct {
	id          int
	conn        Conn
	lockFactory lock.LockFactory
}

func (t *team) ID() int { return t.id }

func (t *team) Workers() ([]Worker, error) {
	return getWorkers(t.conn, workersQuery.Where(sq.Or{
		sq.Eq{"t.id": t.id},
		sq.Eq{"w.team_id": nil},
	}))
}

func (t *team) FindResourceCheckContainer(
	workerName string,
	resourceConfig *UsedResourceConfig,
) (CreatingContainer, CreatedContainer, error) {
	return t.findContainer(sq.And{
		sq.Eq{"worker_name": workerName},
		sq.Eq{"resource_config_id": resourceConfig.ID},
		sq.Or{
			sq.Eq{"best_if_used_by": nil},
			sq.Expr("best_if_used_by > NOW()"),
		},
	})
}

func (t *team) CreateResourceCheckContainer(
	workerName string,
	resourceConfig *UsedResourceConfig,
) (CreatingContainer, error) {
	tx, err := t.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	handle, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}

	brtID := t.findBaseResourceTypeID(resourceConfig)
	wbrtID, err := t.findWorkerBaseResourceType(brtID, workerName, tx)

	var containerID int
	err = psql.Insert("containers").
		Columns(
			"worker_name",
			"resource_config_id",
			"type",
			"step_name",
			"handle",
			"team_id",
			"worker_base_resource_type_id",
		).
		Values(
			workerName,
			resourceConfig.ID,
			"check",
			"",
			handle.String(),
			t.id,
			*wbrtID,
		).
		Suffix("RETURNING id").
		RunWith(tx).
		QueryRow().
		Scan(&containerID)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "foreign_key_violation" {
			return nil, ErrResourceConfigDisappeared
		}

		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &creatingContainer{
		id:         containerID,
		handle:     handle.String(),
		workerName: workerName,
		conn:       t.conn,
	}, nil
}

func (t *team) findBaseResourceTypeID(resourceConfig *UsedResourceConfig) *UsedBaseResourceType {
	if resourceConfig.CreatedByBaseResourceType != nil {
		return resourceConfig.CreatedByBaseResourceType
	} else {
		return t.findBaseResourceTypeID(resourceConfig.CreatedByResourceCache.ResourceConfig)
	}
}

func (t *team) findWorkerBaseResourceType(usedBaseResourceType *UsedBaseResourceType, workerName string, tx Tx) (*int, error) {
	var wbrtID int

	err := psql.Select("id").From("worker_base_resource_types").Where(sq.Eq{
		"worker_name":           workerName,
		"base_resource_type_id": usedBaseResourceType.ID,
	}).RunWith(tx).QueryRow().Scan(&wbrtID)

	if err != nil {
		return nil, err
	}

	return &wbrtID, nil
}

func (t *team) CreateResourceGetContainer(
	workerName string,
	resourceCache *UsedResourceCache,
	stepName string,
) (CreatingContainer, error) {
	var workerResourcCache *UsedWorkerResourceCache
	err := safeFindOrCreate(t.conn, func(tx Tx) error {
		var err error
		workerResourcCache, err = WorkerResourceCache{
			WorkerName:    workerName,
			ResourceCache: resourceCache,
		}.FindOrCreate(tx)
		return err
	})
	if err != nil {
		return nil, err
	}

	tx, err := t.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	handle, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}

	var containerID int
	err = psql.Insert("containers").
		Columns(
			"worker_name",
			"worker_resource_cache_id",
			"type",
			"step_name",
			"handle",
			"team_id",
		).
		Values(
			workerName,
			workerResourcCache.ID,
			"get",
			stepName,
			handle.String(),
			t.id,
		).
		Suffix("RETURNING id").
		RunWith(tx).
		QueryRow().
		Scan(&containerID)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "foreign_key_violation" {
			return nil, ErrResourceCacheDisappeared
		}

		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &creatingContainer{
		id:         containerID,
		handle:     handle.String(),
		workerName: workerName,
		conn:       t.conn,
	}, nil
}

func (t *team) FindBuildContainer(
	workerName string,
	buildID int,
	planID atc.PlanID,
	meta ContainerMetadata,
) (CreatingContainer, CreatedContainer, error) {
	return t.findContainer(sq.And{
		sq.Eq{"worker_name": workerName},
		sq.Eq{"build_id": buildID},
		sq.Eq{"plan_id": string(planID)},
	})
}

func (t *team) CreateBuildContainer(
	workerName string,
	buildID int,
	planID atc.PlanID,
	meta ContainerMetadata,
) (CreatingContainer, error) {
	tx, err := t.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	handle, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}

	var containerID int
	err = psql.Insert("containers").
		Columns(
			"worker_name",
			"build_id",
			"plan_id",
			"type",
			"step_name",
			"handle",
			"team_id",
		).
		Values(
			workerName,
			buildID,
			string(planID),
			meta.Type,
			meta.Name,
			handle.String(),
			t.id,
		).
		Suffix("RETURNING id").
		RunWith(tx).
		QueryRow().
		Scan(&containerID)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "foreign_key_violation" {
			return nil, ErrBuildDisappeared
		}
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &creatingContainer{
		id:         containerID,
		handle:     handle.String(),
		workerName: workerName,
		conn:       t.conn,
	}, nil
}

func (t *team) FindContainerByHandle(
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
	pausedState PipelinePausedState,
) (Pipeline, bool, error) {
	payload, err := json.Marshal(config)
	if err != nil {
		return nil, false, err
	}

	var created bool
	var existingConfig int

	var savedPipeline *pipeline

	tx, err := t.conn.Begin()
	if err != nil {
		return nil, false, err
	}

	defer tx.Rollback()

	err = tx.QueryRow(`
		SELECT COUNT(1)
		FROM pipelines
		WHERE name = $1
	  AND team_id = $2
	`, pipelineName, t.id).Scan(&existingConfig)
	if err != nil {
		return nil, false, err
	}

	if existingConfig == 0 {
		if pausedState == PipelineNoChange {
			pausedState = PipelinePaused
		}

		savedPipeline, err = t.scanPipeline(tx.QueryRow(`
		INSERT INTO pipelines (name, config, version, ordering, paused, team_id)
		VALUES (
			$1,
			$2,
			nextval('config_version_seq'),
			(SELECT COUNT(1) + 1 FROM pipelines),
			$3,
			$4
		)
		RETURNING `+unqualifiedPipelineColumns+`,
		(
			SELECT t.name as team_name FROM teams t WHERE t.id = $4
		)
		`, pipelineName, payload, pausedState.Bool(), t.id))
		if err != nil {
			return nil, false, err
		}

		created = true

		_, err = tx.Exec(fmt.Sprintf(`
		CREATE TABLE pipeline_build_events_%[1]d ()
		INHERITS (build_events);
		`, savedPipeline.ID()))
		if err != nil {
			return nil, false, err
		}

		_, err = tx.Exec(fmt.Sprintf(`
		CREATE INDEX pipeline_build_events_%[1]d_build_id ON pipeline_build_events_%[1]d (build_id);
		`, savedPipeline.ID()))
		if err != nil {
			return nil, false, err
		}

		_, err = tx.Exec(fmt.Sprintf(`
		CREATE UNIQUE INDEX pipeline_build_events_%[1]d_build_id_event_id ON pipeline_build_events_%[1]d (build_id, event_id);
		`, savedPipeline.ID()))
		if err != nil {
			return nil, false, err
		}
	} else {
		if pausedState == PipelineNoChange {
			savedPipeline, err = t.scanPipeline(tx.QueryRow(`
			UPDATE pipelines
			SET config = $1, version = nextval('config_version_seq')
			WHERE name = $2
			AND version = $3
			AND team_id = $4
			RETURNING `+unqualifiedPipelineColumns+`,
			(
				SELECT t.name as team_name FROM teams t WHERE t.id = $4
			)
			`, payload, pipelineName, from, t.id))
		} else {
			savedPipeline, err = t.scanPipeline(tx.QueryRow(`
			UPDATE pipelines
			SET config = $1, version = nextval('config_version_seq'), paused = $2
			WHERE name = $3
			AND version = $4
			AND team_id = $5
			RETURNING `+unqualifiedPipelineColumns+`,
			(
				SELECT t.name as team_name FROM teams t WHERE t.id = $4
			)
			`, payload, pausedState.Bool(), pipelineName, from, t.id))
		}

		if err != nil && err != sql.ErrNoRows {
			return nil, false, err
		}

		if savedPipeline.ID() == 0 {
			return nil, false, ErrConfigComparisonFailed
		}

		_, err = tx.Exec(`
      DELETE FROM jobs_serial_groups
      WHERE job_id in (
        SELECT j.id
        FROM jobs j
        WHERE j.pipeline_id = $1
      )
		`, savedPipeline.ID())
		if err != nil {
			return nil, false, err
		}

		_, err = tx.Exec(`
			UPDATE jobs
			SET active = false
			WHERE pipeline_id = $1
		`, savedPipeline.ID())
		if err != nil {
			return nil, false, err
		}

		_, err = tx.Exec(`
			UPDATE resources
			SET active = false
			WHERE pipeline_id = $1
		`, savedPipeline.ID())
		if err != nil {
			return nil, false, err
		}

		_, err = tx.Exec(`
			UPDATE resource_types
			SET active = false
			WHERE pipeline_id = $1
		`, savedPipeline.ID())
		if err != nil {
			return nil, false, err
		}
	}

	for _, resource := range config.Resources {
		err = t.saveResource(tx, resource, savedPipeline.ID())
		if err != nil {
			return nil, false, err
		}
	}

	for _, resourceType := range config.ResourceTypes {
		err = t.saveResourceType(tx, resourceType, savedPipeline.ID())
		if err != nil {
			return nil, false, err
		}
	}

	for _, job := range config.Jobs {
		err = t.saveJob(tx, job, savedPipeline.ID())
		if err != nil {
			return nil, false, err
		}

		for _, sg := range job.SerialGroups {
			err = t.registerSerialGroup(tx, job.Name, sg, savedPipeline.ID())
			if err != nil {
				return nil, false, err
			}
		}
	}

	err = tx.Commit()
	if err != nil {
		return nil, false, err
	}

	return savedPipeline, created, nil
}

func (t *team) FindPipelineByName(pipelineName string) (Pipeline, bool, error) {
	tx, err := t.conn.Begin()
	if err != nil {
		return nil, false, err
	}

	defer tx.Rollback()

	var pipelineID int
	err = psql.Select("p.id").
		From("pipelines p").
		Join("teams t ON t.id = p.team_id").
		Where(sq.Eq{"p.name": pipelineName}).
		Where(sq.Eq{"team_id": t.id}).
		RunWith(tx).
		QueryRow().
		Scan(&pipelineID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	return &pipeline{
		id:          pipelineID,
		teamID:      t.id,
		conn:        t.conn,
		lockFactory: t.lockFactory,
	}, true, nil
}

func (t *team) CreateOneOffBuild() (Build, error) {
	tx, err := t.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	var buildID int
	err = psql.Insert("builds").
		Columns("team_id", "name", "status").
		Values(t.id, sq.Expr("nextval('one_off_name')"), "pending").
		Suffix("RETURNING id").
		RunWith(tx).
		QueryRow().
		Scan(&buildID)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "foreign_key_violation" {
			return nil, ErrTeamDisappeared
		}
		return nil, err
	}

	err = createBuildEventSeq(tx, buildID)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &build{
		id:     buildID,
		teamID: t.id,
		conn:   t.conn,
	}, nil
}

func (t *team) SaveWorker(atcWorker atc.Worker, ttl time.Duration) (Worker, error) {
	tx, err := t.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	savedWorker, err := saveWorker(tx, atcWorker, &t.id, ttl, t.conn)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &worker{
		name:             savedWorker.Name(),
		state:            WorkerState(savedWorker.State()),
		gardenAddr:       savedWorker.GardenAddr(),
		baggageclaimURL:  &atcWorker.BaggageclaimURL,
		httpProxyURL:     atcWorker.HTTPProxyURL,
		httpsProxyURL:    atcWorker.HTTPSProxyURL,
		noProxy:          atcWorker.NoProxy,
		activeContainers: atcWorker.ActiveContainers,
		resourceTypes:    atcWorker.ResourceTypes,
		platform:         atcWorker.Platform,
		tags:             atcWorker.Tags,
		teamName:         atcWorker.Team,
		teamID:           t.id,
		startTime:        atcWorker.StartTime,
		conn:             t.conn,
	}, nil
}

func (t *team) saveJob(tx Tx, job atc.JobConfig, pipelineID int) error {
	configPayload, err := json.Marshal(job)
	if err != nil {
		return err
	}

	updated, err := checkIfRowsUpdated(tx, `
		UPDATE jobs
		SET config = $3, interruptible = $4, active = true
		WHERE name = $1 AND pipeline_id = $2
	`, job.Name, pipelineID, configPayload, job.Interruptible)
	if err != nil {
		return err
	}

	if updated {
		return nil
	}

	_, err = tx.Exec(`
		INSERT INTO jobs (name, pipeline_id, config, interruptible, active)
		VALUES ($1, $2, $3, $4, true)
	`, job.Name, pipelineID, configPayload, job.Interruptible)

	return swallowUniqueViolation(err)
}

func (t *team) registerSerialGroup(tx Tx, jobName, serialGroup string, pipelineID int) error {
	_, err := tx.Exec(`
    INSERT INTO jobs_serial_groups (serial_group, job_id) VALUES
    ($1, (SELECT j.id
                  FROM jobs j
                       JOIN pipelines p
                         ON j.pipeline_id = p.id
                  WHERE j.name = $2
                    AND j.pipeline_id = $3
                 LIMIT  1));`,
		serialGroup, jobName, pipelineID,
	)

	return swallowUniqueViolation(err)
}

func (t *team) saveResource(tx Tx, resource atc.ResourceConfig, pipelineID int) error {
	configPayload, err := json.Marshal(resource)
	if err != nil {
		return err
	}

	sourceHash := mapHash(resource.Source)

	updated, err := checkIfRowsUpdated(tx, `
		UPDATE resources
		SET config = $3, source_hash=$4, active = true
		WHERE name = $1 AND pipeline_id = $2
	`, resource.Name, pipelineID, configPayload, sourceHash)
	if err != nil {
		return err
	}

	if updated {
		return nil
	}

	_, err = tx.Exec(`
		INSERT INTO resources (name, pipeline_id, config, source_hash, active)
		VALUES ($1, $2, $3, $4, true)
	`, resource.Name, pipelineID, configPayload, sourceHash)

	return swallowUniqueViolation(err)
}

func (t *team) saveResourceType(tx Tx, resourceType atc.ResourceType, pipelineID int) error {
	configPayload, err := json.Marshal(resourceType)
	if err != nil {
		return err
	}

	updated, err := checkIfRowsUpdated(tx, `
		UPDATE resource_types
		SET config = $3, type = $4, active = true
		WHERE name = $1 AND pipeline_id = $2
	`, resourceType.Name, pipelineID, configPayload, resourceType.Type)
	if err != nil {
		return err
	}

	if updated {
		return nil
	}

	_, err = tx.Exec(`
		INSERT INTO resource_types (name, type, pipeline_id, config, active)
		VALUES ($1, $2, $3, $4, true)
	`, resourceType.Name, resourceType.Type, pipelineID, configPayload)

	return swallowUniqueViolation(err)
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

func swallowUniqueViolation(err error) error {
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
	tx, err := t.conn.Begin()
	if err != nil {
		return nil, nil, err
	}

	defer tx.Rollback()

	var containerID int
	var workerName string
	var state string
	var hijacked bool
	var handle string
	err = psql.Select("id, worker_name, state, hijacked, handle").
		From("containers").
		Where(whereClause).
		Where(sq.Eq{"team_id": t.id}).
		RunWith(tx).
		QueryRow().
		Scan(&containerID, &workerName, &state, &hijacked, &handle)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, nil
		}
		return nil, nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, nil, err
	}

	switch state {
	case ContainerStateCreated:
		return nil, &createdContainer{
			id:         containerID,
			handle:     handle,
			workerName: workerName,
			hijacked:   hijacked,
			conn:       t.conn,
		}, nil
	case ContainerStateCreating:
		return &creatingContainer{
			id:         containerID,
			handle:     handle,
			workerName: workerName,
			conn:       t.conn,
		}, nil, nil
	}

	return nil, nil, nil
}

func (t *team) scanPipeline(rows scannable) (*pipeline, error) {
	var id int
	var name string
	var configBlob []byte
	var version int
	var paused bool
	var public bool
	var teamID int
	var teamName string

	err := rows.Scan(&id, &name, &configBlob, &version, &paused, &teamID, &public, &teamName)
	if err != nil {
		return nil, err
	}

	var config atc.Config
	err = json.Unmarshal(configBlob, &config)
	if err != nil {
		return nil, err
	}

	return &pipeline{
		id:     id,
		teamID: teamID,

		conn:        t.conn,
		lockFactory: t.lockFactory,
	}, nil
}
