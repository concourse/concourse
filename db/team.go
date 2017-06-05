package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"

	"golang.org/x/crypto/bcrypt"

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
	Name() string
	Admin() bool

	BasicAuth() *atc.BasicAuth
	Auth() map[string]*json.RawMessage

	Delete() error

	SavePipeline(
		pipelineName string,
		config atc.Config,
		from ConfigVersion,
		pausedState PipelinePausedState,
	) (Pipeline, bool, error)

	Pipeline(pipelineName string) (Pipeline, bool, error)
	Pipelines() ([]Pipeline, error)
	PublicPipelines() ([]Pipeline, error)
	VisiblePipelines() ([]Pipeline, error)
	OrderPipelines([]string) error

	CreateOneOffBuild() (Build, error)
	PrivateAndPublicBuilds(Page) ([]Build, Pagination, error)

	SaveWorker(atcWorker atc.Worker, ttl time.Duration) (Worker, error)
	Workers() ([]Worker, error)

	FindContainerByHandle(string) (Container, bool, error)
	FindContainersByMetadata(ContainerMetadata) ([]Container, error)
	FindCheckContainers(lager.Logger, string, string) ([]Container, error)

	FindCreatedContainerByHandle(string) (CreatedContainer, bool, error)

	FindWorkerForResourceCheckContainer(resourceConfig *UsedResourceConfig) (Worker, bool, error)
	FindResourceCheckContainerOnWorker(workerName string, resourceConfig *UsedResourceConfig) (CreatingContainer, CreatedContainer, error)
	CreateResourceCheckContainer(workerName string, resourceConfig *UsedResourceConfig, meta ContainerMetadata) (CreatingContainer, error)

	CreateResourceGetContainer(workerName string, resourceConfig *UsedResourceCache, meta ContainerMetadata) (CreatingContainer, error)

	FindWorkerForContainer(handle string) (Worker, bool, error)
	FindWorkerForBuildContainer(buildID int, planID atc.PlanID) (Worker, bool, error)
	FindBuildContainerOnWorker(workerName string, buildID int, planID atc.PlanID) (CreatingContainer, CreatedContainer, error)
	CreateBuildContainer(workerName string, buildID int, planID atc.PlanID, meta ContainerMetadata) (CreatingContainer, error)

	FindWorkerForContainerByOwner(ContainerOwner) (Worker, bool, error)
	FindContainerOnWorker(workerName string, owner ContainerOwner) (CreatingContainer, CreatedContainer, error)
	CreateContainer(workerName string, owner ContainerOwner, meta ContainerMetadata) (CreatingContainer, error)

	UpdateBasicAuth(basicAuth *atc.BasicAuth) error
	UpdateProviderAuth(auth map[string]*json.RawMessage) error

	CreatePipe(string, string) error
	GetPipe(string) (Pipe, error)
}

type team struct {
	id          int
	conn        Conn
	lockFactory lock.LockFactory

	name  string
	admin bool

	basicAuth *atc.BasicAuth

	auth map[string]*json.RawMessage
}

func (t *team) ID() int                           { return t.id }
func (t *team) Name() string                      { return t.name }
func (t *team) Admin() bool                       { return t.admin }
func (t *team) BasicAuth() *atc.BasicAuth         { return t.basicAuth }
func (t *team) Auth() map[string]*json.RawMessage { return t.auth }

func (t *team) Delete() error {
	tx, err := t.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	_, err = psql.Delete("teams").
		Where(sq.Eq{
			"name": t.name,
		}).
		RunWith(tx).
		Exec()

	if err != nil {
		return err
	}

	teamBuildEvents := fmt.Sprintf("team_build_events_%d", int64(t.id))
	_, err = psql.Delete(teamBuildEvents).
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

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

func (t *team) FindWorkerForContainerByOwner(owner ContainerOwner) (Worker, bool, error) {
	ownerEq := sq.Eq{}
	for k, v := range owner.SetMap() {
		ownerEq["c."+k] = v
	}

	return getWorker(t.conn, workersQuery.Join("containers c ON c.worker_name = w.name").Where(sq.And{
		sq.Eq{"c.team_id": t.id},
		ownerEq,
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

func (t *team) FindContainerOnWorker(workerName string, owner ContainerOwner) (CreatingContainer, CreatedContainer, error) {
	return t.findContainer(sq.And{
		sq.Eq{"worker_name": workerName},
		sq.Eq(owner.SetMap()),
	})
}

func (t *team) CreateContainer(workerName string, owner ContainerOwner, meta ContainerMetadata) (CreatingContainer, error) {
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

	for k, v := range owner.SetMap() {
		insMap[k] = v
	}

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
			containers = append(containers, destroying)
		}
	}

	return containers, nil
}

func (t *team) FindCheckContainers(logger lager.Logger, pipelineName string, resourceName string) ([]Container, error) {
	pipeline, found, err := t.Pipeline(pipelineName)
	if err != nil {
		return nil, err
	}
	if !found {
		return []Container{}, nil
	}

	resource, found, err := pipeline.Resource(resourceName)
	if err != nil {
		return nil, err
	}
	if !found {
		return []Container{}, nil
	}

	pipelineResourceTypes, err := pipeline.ResourceTypes()
	if err != nil {
		return nil, err
	}

	resourceConfigFactory := NewResourceConfigFactory(t.conn, t.lockFactory)
	resourceConfig, found, err := resourceConfigFactory.FindResourceConfig(
		logger,
		resource.Type(),
		resource.Source(),
		pipelineResourceTypes.Deserialize(),
	)
	if err != nil {
		return nil, err
	}
	if !found {
		return []Container{}, nil
	}

	rows, err := selectContainers().
		Where(sq.Eq{
			"resource_config_id": resourceConfig.ID,
			"team_id":            t.id,
		}).
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
			containers = append(containers, destroying)
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

	es := t.conn.EncryptionStrategy()
	encryptedPayload, nonce, err := es.Encrypt(payload)
	if err != nil {
		return nil, false, err
	}

	var created bool
	var existingConfig int

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

	var pipelineID int
	if existingConfig == 0 {
		if pausedState == PipelineNoChange {
			pausedState = PipelinePaused
		}

		err = psql.Insert("pipelines").
			SetMap(map[string]interface{}{
				"name":     pipelineName,
				"config":   encryptedPayload,
				"version":  sq.Expr("nextval('config_version_seq')"),
				"ordering": sq.Expr("(SELECT COUNT(1) + 1 FROM pipelines)"),
				"paused":   pausedState.Bool(),
				"team_id":  t.id,
				"nonce":    nonce,
			}).
			Suffix("RETURNING id").
			RunWith(tx).
			QueryRow().Scan(&pipelineID)
		if err != nil {
			return nil, false, err
		}

		created = true

		_, err = tx.Exec(fmt.Sprintf(`
			CREATE TABLE pipeline_build_events_%[1]d ()
			INHERITS (build_events)
		`, pipelineID))
		if err != nil {
			return nil, false, err
		}

		_, err = tx.Exec(fmt.Sprintf(`
			CREATE INDEX pipeline_build_events_%[1]d_build_id ON pipeline_build_events_%[1]d (build_id)
		`, pipelineID))
		if err != nil {
			return nil, false, err
		}

		_, err = tx.Exec(fmt.Sprintf(`
			CREATE UNIQUE INDEX pipeline_build_events_%[1]d_build_id_event_id ON pipeline_build_events_%[1]d (build_id, event_id)
		`, pipelineID))
		if err != nil {
			return nil, false, err
		}
	} else {
		update := psql.Update("pipelines").
			Set("config", encryptedPayload).
			Set("version", sq.Expr("nextval('config_version_seq')")).
			Set("nonce", nonce).
			Where(sq.Eq{
				"name":    pipelineName,
				"version": from,
				"team_id": t.id,
			}).
			Suffix("RETURNING id")

		if pausedState != PipelineNoChange {
			update = update.Set("paused", pausedState.Bool())
		}

		err = update.RunWith(tx).QueryRow().Scan(&pipelineID)
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
		`, pipelineID)
		if err != nil {
			return nil, false, err
		}

		_, err = tx.Exec(`
			UPDATE jobs
			SET active = false
			WHERE pipeline_id = $1
		`, pipelineID)
		if err != nil {
			return nil, false, err
		}

		_, err = tx.Exec(`
			UPDATE resources
			SET active = false
			WHERE pipeline_id = $1
		`, pipelineID)
		if err != nil {
			return nil, false, err
		}

		_, err = tx.Exec(`
			UPDATE resource_types
			SET active = false
			WHERE pipeline_id = $1
		`, pipelineID)
		if err != nil {
			return nil, false, err
		}
	}

	for _, resource := range config.Resources {
		err = t.saveResource(tx, resource, pipelineID)
		if err != nil {
			return nil, false, err
		}
	}

	for _, resourceType := range config.ResourceTypes {
		err = t.saveResourceType(tx, resourceType, pipelineID)
		if err != nil {
			return nil, false, err
		}
	}

	for _, job := range config.Jobs {
		err = t.saveJob(tx, job, pipelineID)
		if err != nil {
			return nil, false, err
		}

		for _, sg := range job.SerialGroups {
			err = t.registerSerialGroup(tx, job.Name, sg, pipelineID)
			if err != nil {
				return nil, false, err
			}
		}
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

	return pipeline, created, nil
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

func (t *team) VisiblePipelines() ([]Pipeline, error) {
	rows, err := pipelinesQuery.
		Where(sq.Eq{"team_id": t.id}).
		OrderBy("ordering").
		RunWith(t.conn).
		Query()
	if err != nil {
		return nil, err
	}

	currentTeamPipelines, err := scanPipelines(t.conn, t.lockFactory, rows)
	if err != nil {
		return nil, err
	}

	rows, err = pipelinesQuery.
		Where(sq.NotEq{"team_id": t.id}).
		Where(sq.Eq{"public": true}).
		OrderBy("ordering").
		RunWith(t.conn).
		Query()
	if err != nil {
		return nil, err
	}

	otherTeamPublicPipelines, err := scanPipelines(t.conn, t.lockFactory, rows)
	if err != nil {
		return nil, err
	}

	return append(currentTeamPipelines, otherTeamPublicPipelines...), nil
}

func (t *team) OrderPipelines(pipelineNames []string) error {
	tx, err := t.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	var pipelineCount int

	err = psql.Select("COUNT(1)").
		From("pipelines").
		Where(sq.Eq{"team_id": t.id}).
		RunWith(tx).
		QueryRow().
		Scan(&pipelineCount)
	if err != nil {
		return err
	}

	_, err = psql.Update("pipelines").
		Set("ordering", pipelineCount+1).
		Where(sq.Eq{"team_id": t.id}).
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	for i, name := range pipelineNames {
		_, err = psql.Update("pipelines").
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
	}

	return tx.Commit()
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

	build := &build{conn: t.conn, lockFactory: t.lockFactory}
	err = scanBuild(build, buildsQuery.
		Where(sq.Eq{"b.id": buildID}).
		RunWith(tx).
		QueryRow(),
	)
	if err != nil {
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

	return build, nil
}

func (t *team) PrivateAndPublicBuilds(page Page) ([]Build, Pagination, error) {
	newBuildsQuery := buildsQuery.
		Where(sq.Or{sq.Eq{"p.public": true}, sq.Eq{"t.id": t.id}})

	return getBuildsWithPagination(newBuildsQuery, page, t.conn, t.lockFactory)
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

func (t *team) UpdateBasicAuth(basicAuth *atc.BasicAuth) error {
	encryptedBasicAuth, err := encryptedJSON(basicAuth)
	if err != nil {
		return err
	}

	query := `
		UPDATE teams
		SET basic_auth = $1
		WHERE LOWER(name) = LOWER($2)
		RETURNING id, name, admin, basic_auth, auth, nonce
	`

	params := []interface{}{encryptedBasicAuth, t.name}

	return t.queryTeam(query, params)
}

func (t *team) UpdateProviderAuth(auth map[string]*json.RawMessage) error {
	jsonEncodedProviderAuth, err := json.Marshal(auth)
	if err != nil {
		return err
	}

	es := t.conn.EncryptionStrategy()
	encryptedAuth, nonce, err := es.Encrypt(jsonEncodedProviderAuth)
	if err != nil {
		return err
	}

	query := `
		UPDATE teams
		SET auth = $1, nonce = $3
		WHERE LOWER(name) = LOWER($2)
		RETURNING id, name, admin, basic_auth, auth, nonce
	`
	params := []interface{}{string(encryptedAuth), t.name, nonce}
	return t.queryTeam(query, params)
}

func (t *team) CreatePipe(pipeGUID string, url string) error {
	tx, err := t.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO pipes(id, url, team_id)
		VALUES (
			$1,
			$2,
			( SELECT id
				FROM teams
				WHERE name = $3
			)
		)
	`, pipeGUID, url, t.name)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (t *team) GetPipe(pipeGUID string) (Pipe, error) {
	tx, err := t.conn.Begin()
	if err != nil {
		return Pipe{}, err
	}

	defer tx.Rollback()

	var pipe Pipe

	err = tx.QueryRow(`
		SELECT p.id AS pipe_id, coalesce(url, '') AS url, t.name AS team_name
		FROM pipes p
			JOIN teams t
			ON t.id = p.team_id
		WHERE p.id = $1
	`, pipeGUID).Scan(&pipe.ID, &pipe.URL, &pipe.TeamName)
	if err != nil {
		return Pipe{}, err
	}

	err = tx.Commit()
	if err != nil {
		return Pipe{}, err
	}

	return pipe, nil
}

func (t *team) saveJob(tx Tx, job atc.JobConfig, pipelineID int) error {
	configPayload, err := json.Marshal(job)
	if err != nil {
		return err
	}

	es := t.conn.EncryptionStrategy()
	encryptedPayload, nonce, err := es.Encrypt(configPayload)
	if err != nil {
		return err
	}

	updated, err := checkIfRowsUpdated(tx, `
		UPDATE jobs
		SET config = $3, interruptible = $4, active = true, nonce = $5
		WHERE name = $1 AND pipeline_id = $2
	`, job.Name, pipelineID, encryptedPayload, job.Interruptible, nonce)
	if err != nil {
		return err
	}

	if updated {
		return nil
	}

	_, err = tx.Exec(`
		INSERT INTO jobs (name, pipeline_id, config, interruptible, active, nonce)
		VALUES ($1, $2, $3, $4, true, $5)
	`, job.Name, pipelineID, encryptedPayload, job.Interruptible, nonce)

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

	es := t.conn.EncryptionStrategy()
	encryptedPayload, nonce, err := es.Encrypt(configPayload)
	if err != nil {
		return err
	}

	sourceHash := mapHash(resource.Source)

	updated, err := checkIfRowsUpdated(tx, `
		UPDATE resources
		SET config = $3, source_hash=$4, active = true, nonce = $5
		WHERE name = $1 AND pipeline_id = $2
	`, resource.Name, pipelineID, encryptedPayload, sourceHash, nonce)
	if err != nil {
		return err
	}

	if updated {
		return nil
	}

	_, err = tx.Exec(`
		INSERT INTO resources (name, pipeline_id, config, source_hash, active, nonce)
		VALUES ($1, $2, $3, $4, true, $5)
	`, resource.Name, pipelineID, encryptedPayload, sourceHash, nonce)

	return swallowUniqueViolation(err)
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

	updated, err := checkIfRowsUpdated(tx, `
		UPDATE resource_types
		SET config = $3, type = $4, active = true, nonce = $5
		WHERE name = $1 AND pipeline_id = $2
	`, resourceType.Name, pipelineID, encryptedPayload, resourceType.Type, nonce)
	if err != nil {
		return err
	}

	if updated {
		return nil
	}

	_, err = tx.Exec(`
		INSERT INTO resource_types (name, type, pipeline_id, config, active, nonce)
		VALUES ($1, $2, $3, $4, true, $5)
	`, resourceType.Name, resourceType.Type, pipelineID, encryptedPayload, nonce)

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

func scanPipeline(p *pipeline, scan scannable) error {
	err := scan.Scan(&p.id, &p.name, &p.configVersion, &p.teamID, &p.teamName, &p.paused, &p.public)
	return err
}

func scanPipelines(conn Conn, lockFactory lock.LockFactory, rows *sql.Rows) ([]Pipeline, error) {
	defer rows.Close()

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

func (t *team) queryTeam(query string, params []interface{}) error {
	var basicAuth, providerAuth, nonce sql.NullString

	tx, err := t.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = tx.QueryRow(query, params...).Scan(
		&t.id,
		&t.name,
		&t.admin,
		&basicAuth,
		&providerAuth,
		&nonce,
	)
	if err != nil {
		return err
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	if basicAuth.Valid {
		err = json.Unmarshal([]byte(basicAuth.String), &t.basicAuth)

		if err != nil {
			return err
		}
	}

	if providerAuth.Valid {
		es := t.conn.EncryptionStrategy()

		var noncense *string
		if nonce.Valid {
			noncense = &nonce.String
		}

		pAuth, err := es.Decrypt(providerAuth.String, noncense)
		if err != nil {
			return err
		}

		err = json.Unmarshal(pAuth, &t.auth)
		if err != nil {
			return err
		}
	}

	return nil
}

func encryptedJSON(b *atc.BasicAuth) (string, error) {
	var result *atc.BasicAuth
	if b != nil && b.BasicAuthUsername != "" && b.BasicAuthPassword != "" {
		encryptedPw, err := bcrypt.GenerateFromPassword([]byte(b.BasicAuthPassword), 4)
		if err != nil {
			return "", err
		}
		result = &atc.BasicAuth{
			BasicAuthPassword: string(encryptedPw),
			BasicAuthUsername: b.BasicAuthUsername,
		}
	}

	json, err := json.Marshal(result)
	return string(json), err
}
