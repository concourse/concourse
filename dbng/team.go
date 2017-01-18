package dbng

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/lib/pq"
	uuid "github.com/nu7hatch/gouuid"
)

var ErrConfigComparisonFailed = errors.New("comparison with existing config failed during save")

//go:generate counterfeiter . Team

type Team interface {
	ID() int
	SavePipeline(
		pipelineName string,
		config atc.Config,
		from ConfigVersion,
		pausedState PipelinePausedState,
	) (Pipeline, bool, error)

	CreateOneOffBuild() (*Build, error)

	SaveWorker(worker atc.Worker, ttl time.Duration) (*Worker, error)

	FindContainerByHandle(string) (CreatedContainer, bool, error)

	FindResourceCheckContainer(*Worker, *UsedResourceConfig) (CreatingContainer, CreatedContainer, error)
	CreateResourceCheckContainer(*Worker, *UsedResourceConfig) (CreatingContainer, error)

	FindResourceGetContainer(*Worker, *UsedResourceCache, string) (CreatingContainer, CreatedContainer, error)
	CreateResourceGetContainer(*Worker, *UsedResourceCache, string) (CreatingContainer, error)

	FindBuildContainer(*Worker, *Build, atc.PlanID, ContainerMetadata) (CreatingContainer, CreatedContainer, error)
	CreateBuildContainer(*Worker, *Build, atc.PlanID, ContainerMetadata) (CreatingContainer, error)
}

type team struct {
	id   int
	conn Conn
}

func (t *team) ID() int { return t.id }

func (t *team) FindResourceCheckContainer(
	worker *Worker,
	resourceConfig *UsedResourceConfig,
) (CreatingContainer, CreatedContainer, error) {
	return t.findContainer(map[string]interface{}{
		"worker_name":        worker.Name,
		"resource_config_id": resourceConfig.ID,
	})
}

func (t *team) CreateResourceCheckContainer(
	worker *Worker,
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

	var containerID int
	err = psql.Insert("containers").
		Columns(
			"worker_name",
			"resource_config_id",
			"type",
			"step_name",
			"handle",
			"team_id",
		).
		Values(
			worker.Name,
			resourceConfig.ID,
			"check",
			"",
			handle.String(),
			t.id,
		).
		Suffix("RETURNING id").
		RunWith(tx).
		QueryRow().
		Scan(&containerID)
	if err != nil {
		// TODO: explicitly handle fkey constraint
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &creatingContainer{
		id:         containerID,
		handle:     handle.String(),
		workerName: worker.Name,
		conn:       t.conn,
	}, nil
}

func (t *team) FindResourceGetContainer(
	worker *Worker,
	resourceCache *UsedResourceCache,
	stepName string,
) (CreatingContainer, CreatedContainer, error) {
	return t.findContainer(map[string]interface{}{
		"worker_name":       worker.Name,
		"resource_cache_id": resourceCache.ID,
		"step_name":         stepName,
	})
}

func (t *team) CreateResourceGetContainer(
	worker *Worker,
	resourceCache *UsedResourceCache,
	stepName string,
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
			"resource_cache_id",
			"type",
			"step_name",
			"handle",
			"team_id",
		).
		Values(
			worker.Name,
			resourceCache.ID,
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
		// TODO: explicitly handle fkey constraint
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &creatingContainer{
		id:         containerID,
		handle:     handle.String(),
		workerName: worker.Name,
		conn:       t.conn,
	}, nil
}

func (t *team) FindBuildContainer(
	worker *Worker,
	build *Build,
	planID atc.PlanID,
	meta ContainerMetadata,
) (CreatingContainer, CreatedContainer, error) {
	return t.findContainer(map[string]interface{}{
		"worker_name": worker.Name,
		"build_id":    build.ID,
		"plan_id":     string(planID),
		"type":        meta.Type,
		"step_name":   meta.Name,
	})
}

func (t *team) CreateBuildContainer(
	worker *Worker,
	build *Build,
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
		// TODO: should metadata just be JSON?
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
			worker.Name,
			build.ID,
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
		// TODO: explicitly handle fkey constraint
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &creatingContainer{
		id:         containerID,
		handle:     handle.String(),
		workerName: worker.Name,
		conn:       t.conn,
	}, nil
}

func (t *team) FindContainerByHandle(
	handle string,
) (CreatedContainer, bool, error) {
	_, createdContainer, err := t.findContainer(map[string]interface{}{
		"handle": handle,
	})
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

		savedPipeline, err = scanPipeline(tx.QueryRow(`
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
		`, pipelineName, payload, pausedState.Bool(), t.id), t.conn)
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
			savedPipeline, err = scanPipeline(tx.QueryRow(`
			UPDATE pipelines
			SET config = $1, version = nextval('config_version_seq')
			WHERE name = $2
			AND version = $3
			AND team_id = $4
			RETURNING `+unqualifiedPipelineColumns+`,
			(
				SELECT t.name as team_name FROM teams t WHERE t.id = $4
			)
			`, payload, pipelineName, from, t.id), t.conn)
		} else {
			savedPipeline, err = scanPipeline(tx.QueryRow(`
			UPDATE pipelines
			SET config = $1, version = nextval('config_version_seq'), paused = $2
			WHERE name = $3
			AND version = $4
			AND team_id = $5
			RETURNING `+unqualifiedPipelineColumns+`,
			(
				SELECT t.name as team_name FROM teams t WHERE t.id = $4
			)
			`, payload, pausedState.Bool(), pipelineName, from, t.id), t.conn)
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

func (t *team) CreateOneOffBuild() (*Build, error) {
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
		// TODO: explicitly handle fkey constraint
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

	return &Build{
		ID:     buildID,
		teamID: t.id,
	}, nil
}

func (t *team) SaveWorker(worker atc.Worker, ttl time.Duration) (*Worker, error) {
	tx, err := t.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	savedWorker, err := saveWorker(tx, worker, &t.id, ttl)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return savedWorker, nil
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

	updated, err := checkIfRowsUpdated(tx, `
		UPDATE resources
		SET config = $3, active = true
		WHERE name = $1 AND pipeline_id = $2
	`, resource.Name, pipelineID, configPayload)
	if err != nil {
		return err
	}

	if updated {
		return nil
	}

	_, err = tx.Exec(`
		INSERT INTO resources (name, pipeline_id, config, active)
		VALUES ($1, $2, $3, true)
	`, resource.Name, pipelineID, configPayload)

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

func (t *team) findContainer(columns map[string]interface{}) (CreatingContainer, CreatedContainer, error) {
	tx, err := t.conn.Begin()
	if err != nil {
		return nil, nil, err
	}

	defer tx.Rollback()

	whereClause := sq.Eq{}
	for name, value := range columns {
		whereClause[name] = value
	}

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
