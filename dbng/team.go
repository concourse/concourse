package dbng

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db/lock"
	"github.com/lib/pq"
	uuid "github.com/nu7hatch/gouuid"
)

var ErrConfigComparisonFailed = errors.New("comparison with existing config failed during save")
var ErrTeamDisappeared = errors.New("team disappeared")

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

	FindContainerByHandle(string) (Container, bool, error)
	FindContainersByMetadata(ContainerMetadata) ([]Container, error)

	FindCreatedContainerByHandle(string) (CreatedContainer, bool, error)

	FindWorkerForResourceCheckContainer(resourceConfig *UsedResourceConfig) (Worker, bool, error)
	FindResourceCheckContainerOnWorker(workerName string, resourceConfig *UsedResourceConfig) (CreatingContainer, CreatedContainer, error)
	CreateResourceCheckContainer(workerName string, resourceConfig *UsedResourceConfig, meta ContainerMetadata) (CreatingContainer, error)

	CreateResourceGetContainer(workerName string, resourceConfig *UsedResourceCache, meta ContainerMetadata) (CreatingContainer, error)

	FindWorkerForContainer(handle string) (Worker, bool, error)
	FindWorkerForBuildContainer(buildID int, planID atc.PlanID) (Worker, bool, error)
	FindBuildContainerOnWorker(workerName string, buildID int, planID atc.PlanID) (CreatingContainer, CreatedContainer, error)
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

func (t *team) FindWorkerForResourceCheckContainer(
	resourceConfig *UsedResourceConfig,
) (Worker, bool, error) {
	return getWorker(t.conn, workersQuery.Join("containers c ON c.worker_name = w.name").Where(sq.And{
		sq.Eq{"c.resource_config_id": resourceConfig.ID},
		sq.Or{
			sq.Eq{"c.best_if_used_by": nil},
			sq.Expr("c.best_if_used_by > NOW()"),
		},
		sq.Eq{"c.team_id": t.id},
	}))
}

func (t *team) FindResourceCheckContainerOnWorker(
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

	brtID := t.findBaseResourceTypeID(resourceConfig)
	wbrtID, err := t.findWorkerBaseResourceType(brtID, workerName, tx)
	if err != nil {
		return nil, err
	}

	var containerID int
	cols := []interface{}{&containerID}

	metadata := &ContainerMetadata{}
	cols = append(cols, metadata.ScanTargets()...)

	var biub time.Time
	err = psql.Select("NOW() + LEAST(GREATEST('5 minutes'::interval, NOW() - to_timestamp(w.start_time)), '1 hour'::interval)").
		From("workers w").
		Where(sq.Eq{"w.name": workerName}).
		RunWith(tx).
		QueryRow().
		Scan(&biub)
	if err != nil {
		return nil, err
	}

	insMap := meta.SQLMap()
	insMap["worker_name"] = workerName
	insMap["handle"] = handle.String()
	insMap["team_id"] = t.id
	insMap["resource_config_id"] = resourceConfig.ID
	insMap["worker_base_resource_type_id"] = wbrtID
	insMap["best_if_used_by"] = biub

	err = psql.Insert("containers").
		SetMap(insMap).
		Suffix("RETURNING id, " + strings.Join(containerMetadataColumns, ", ")).
		RunWith(tx).
		QueryRow().
		Scan(cols...)
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

	return newCreatingContainer(
		containerID,
		handle.String(),
		workerName,
		*metadata,
		t.conn,
	), nil
}

func (t *team) CreateResourceGetContainer(
	workerName string,
	resourceCache *UsedResourceCache,
	meta ContainerMetadata,
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

	handle, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}

	var containerID int
	cols := []interface{}{&containerID}

	metadata := &ContainerMetadata{}
	cols = append(cols, metadata.ScanTargets()...)

	insMap := meta.SQLMap()
	insMap["worker_name"] = workerName
	insMap["handle"] = handle.String()
	insMap["team_id"] = t.id
	insMap["worker_resource_cache_id"] = workerResourcCache.ID

	err = psql.Insert("containers").
		SetMap(insMap).
		Suffix("RETURNING id, " + strings.Join(containerMetadataColumns, ", ")).
		RunWith(t.conn).
		QueryRow().
		Scan(cols...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "foreign_key_violation" {
			return nil, ErrResourceCacheDisappeared
		}

		return nil, err
	}

	return newCreatingContainer(
		containerID,
		handle.String(),
		workerName,
		*metadata,
		t.conn,
	), nil
}

func (t *team) FindWorkerForContainer(handle string) (Worker, bool, error) {
	return getWorker(t.conn, workersQuery.Join("containers c ON c.worker_name = w.name").Where(sq.And{
		sq.Eq{"c.handle": handle},
		sq.Eq{"c.team_id": t.id},
	}))
}

func (t *team) FindWorkerForBuildContainer(
	buildID int,
	planID atc.PlanID,
) (Worker, bool, error) {
	return getWorker(t.conn, workersQuery.Join("containers c ON c.worker_name = w.name").Where(sq.And{
		sq.Eq{"c.build_id": buildID},
		sq.Eq{"c.plan_id": string(planID)},
		sq.Eq{"c.team_id": t.id},
	}))
}

func (t *team) FindBuildContainerOnWorker(
	workerName string,
	buildID int,
	planID atc.PlanID,
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
	handle, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}

	var containerID int
	cols := []interface{}{&containerID}

	metadata := &ContainerMetadata{}
	cols = append(cols, metadata.ScanTargets()...)

	insMap := meta.SQLMap()
	insMap["worker_name"] = workerName
	insMap["handle"] = handle.String()
	insMap["team_id"] = t.id
	insMap["build_id"] = buildID
	insMap["plan_id"] = string(planID)

	err = psql.Insert("containers").
		SetMap(insMap).
		Suffix("RETURNING id, " + strings.Join(containerMetadataColumns, ", ")).
		RunWith(t.conn).
		QueryRow().
		Scan(cols...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "foreign_key_violation" {
			return nil, ErrBuildDisappeared
		}

		return nil, err
	}

	return newCreatingContainer(
		containerID,
		handle.String(),
		workerName,
		*metadata,
		t.conn,
	), nil
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

	defer rows.Close()

	var containers []Container
	for rows.Next() {
		creating, created, destroying, err := scanContainer(rows, t.conn)
		if err != nil {
			return nil, err
		}

		if creating != nil {
			containers = append(containers, creating)
		}

		if created != nil {
			containers = append(containers, created)
		}

		if destroying != nil {
			continue
		}
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

		savedPipeline, err = scanPipeline(t.conn, t.lockFactory, tx.QueryRow(`
			INSERT INTO pipelines (name, config, version, ordering, paused, team_id)
			VALUES (
				$1,
				$2,
				nextval('config_version_seq'),
				(SELECT COUNT(1) + 1 FROM pipelines),
				$3,
				$4
			)
			RETURNING `+unqualifiedPipelineColumns+`
		`, pipelineName, payload, pausedState.Bool(), t.id))
		if err != nil {
			return nil, false, err
		}

		created = true

		_, err = tx.Exec(fmt.Sprintf(`
			CREATE TABLE pipeline_build_events_%[1]d ()
			INHERITS (build_events)
		`, savedPipeline.ID()))
		if err != nil {
			return nil, false, err
		}

		_, err = tx.Exec(fmt.Sprintf(`
			CREATE INDEX pipeline_build_events_%[1]d_build_id ON pipeline_build_events_%[1]d (build_id)
		`, savedPipeline.ID()))
		if err != nil {
			return nil, false, err
		}

		_, err = tx.Exec(fmt.Sprintf(`
			CREATE UNIQUE INDEX pipeline_build_events_%[1]d_build_id_event_id ON pipeline_build_events_%[1]d (build_id, event_id)
		`, savedPipeline.ID()))
		if err != nil {
			return nil, false, err
		}
	} else {
		if pausedState == PipelineNoChange {
			savedPipeline, err = scanPipeline(t.conn, t.lockFactory, tx.QueryRow(`
				UPDATE pipelines
				SET config = $1, version = nextval('config_version_seq')
				WHERE name = $2
				AND version = $3
				AND team_id = $4
				RETURNING `+unqualifiedPipelineColumns+`
			`, payload, pipelineName, from, t.id))
		} else {
			savedPipeline, err = scanPipeline(t.conn, t.lockFactory, tx.QueryRow(`
				UPDATE pipelines
				SET config = $1, version = nextval('config_version_seq'), paused = $2
				WHERE name = $3
				AND version = $4
				AND team_id = $5
				RETURNING `+unqualifiedPipelineColumns+`
			`, payload, pausedState.Bool(), pipelineName, from, t.id))
		}

		if err != nil {
			if err == sql.ErrNoRows {
				return nil, false, ErrConfigComparisonFailed
			}

			return nil, false, err
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
	pipeline, err := scanPipeline(
		t.conn,
		t.lockFactory,
		psql.Select(unqualifiedPipelineColumns).
			From("pipelines").
			Where(sq.Eq{
				"team_id": t.id,
				"name":    pipelineName,
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
	creating, created, destroying, err := scanContainer(
		selectContainers().
			Where(whereClause).
			Where(sq.Eq{"team_id": t.id}).
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

func scanPipeline(conn Conn, lockFactory lock.LockFactory, rows scannable) (*pipeline, error) {
	var id int
	var name string
	var teamID int
	var configVersion int

	err := rows.Scan(&id, &name, &configVersion, &teamID)
	if err != nil {
		return nil, err
	}

	return &pipeline{
		id:     id,
		name:   name,
		teamID: teamID,

		configVersion: ConfigVersion(configVersion),

		conn:        conn,
		lockFactory: lockFactory,
	}, nil
}

func (t *team) findBaseResourceTypeID(resourceConfig *UsedResourceConfig) *UsedBaseResourceType {
	if resourceConfig.CreatedByBaseResourceType != nil {
		return resourceConfig.CreatedByBaseResourceType
	} else {
		return t.findBaseResourceTypeID(resourceConfig.CreatedByResourceCache.ResourceConfig)
	}
}

func (t *team) findWorkerBaseResourceType(usedBaseResourceType *UsedBaseResourceType, workerName string, tx Tx) (int, error) {
	var wbrtID int
	err := psql.Select("id").From("worker_base_resource_types").Where(sq.Eq{
		"worker_name":           workerName,
		"base_resource_type_id": usedBaseResourceType.ID,
	}).RunWith(tx).QueryRow().Scan(&wbrtID)
	if err != nil {
		return 0, err
	}

	return wbrtID, nil
}
