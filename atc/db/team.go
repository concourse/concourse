package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"code.cloudfoundry.org/lager"
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/lib/pq"
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
		pausedState PipelinePausedState,
	) (Pipeline, bool, error)

	Pipeline(pipelineName string) (Pipeline, bool, error)
	Pipelines() ([]Pipeline, error)
	PublicPipelines() ([]Pipeline, error)
	VisiblePipelines() ([]Pipeline, error)
	OrderPipelines([]string) error

	CreateOneOffBuild() (Build, error)
	PrivateAndPublicBuilds(Page) ([]Build, Pagination, error)
	Builds(page Page) ([]Build, Pagination, error)
	BuildsWithTime(page Page) ([]Build, Pagination, error)

	SaveWorker(atcWorker atc.Worker, ttl time.Duration) (Worker, error)
	Workers() ([]Worker, error)

	Containers(lager.Logger) ([]Container, error)
	IsCheckContainer(string) (bool, error)
	IsContainerWithinTeam(string, bool) (bool, error)
	FindContainerByHandle(string) (Container, bool, error)
	FindCheckContainers(lager.Logger, string, string, creds.VariablesFactory) ([]Container, map[int]time.Time, error)
	FindContainersByMetadata(ContainerMetadata) ([]Container, error)
	FindCreatedContainerByHandle(string) (CreatedContainer, bool, error)
	FindWorkerForContainer(handle string) (Worker, bool, error)

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

func (t *team) FindWorkerForContainer(handle string) (Worker, bool, error) {
	return getWorker(t.conn, workersQuery.Join("containers c ON c.worker_name = w.name").Where(sq.And{
		sq.Eq{"c.handle": handle},
	}))
}

func (t *team) Containers(
	logger lager.Logger,
) ([]Container, error) {
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
	pausedState PipelinePausedState,
) (Pipeline, bool, error) {
	groupsPayload, err := json.Marshal(config.Groups)
	if err != nil {
		return nil, false, err
	}

	jobGroups := make(map[string][]string)
	for _, group := range config.Groups {
		for _, job := range group.Jobs {
			jobGroups[job] = append(jobGroups[job], group.Name)
		}
	}

	var created bool
	var existingConfig int

	tx, err := t.conn.Begin()
	if err != nil {
		return nil, false, err
	}

	defer Rollback(tx)

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
				"groups":   groupsPayload,
				"version":  sq.Expr("nextval('config_version_seq')"),
				"ordering": sq.Expr("currval('pipelines_id_seq')"),
				"paused":   pausedState.Bool(),
				"team_id":  t.id,
			}).
			Suffix("RETURNING id").
			RunWith(tx).
			QueryRow().Scan(&pipelineID)
		if err != nil {
			return nil, false, err
		}

		created = true
	} else {
		update := psql.Update("pipelines").
			Set("groups", groupsPayload).
			Set("version", sq.Expr("nextval('config_version_seq')")).
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

	_, err = tx.Exec(`
			UPDATE resources
			SET resource_config_id = NULL
			WHERE pipeline_id = $1
			AND active = false
		`, pipelineID)
	if err != nil {
		return nil, false, err
	}

	for _, resourceType := range config.ResourceTypes {
		err = t.saveResourceType(tx, resourceType, pipelineID)
		if err != nil {
			return nil, false, err
		}
	}

	for _, job := range config.Jobs {
		err = t.saveJob(tx, job, pipelineID, jobGroups[job.Name])
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

	err = removeUnusedWorkerTaskCaches(tx, pipelineID, config.Jobs)
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

func (t *team) VisiblePipelines() ([]Pipeline, error) {
	rows, err := pipelinesQuery.
		Where(sq.Eq{"team_id": t.id}).
		OrderBy("team_id ASC", "ordering ASC").
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
		OrderBy("team_id ASC", "ordering ASC").
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

func (t *team) CreateOneOffBuild() (Build, error) {
	tx, err := t.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	build := &build{conn: t.conn, lockFactory: t.lockFactory}
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

func (t *team) FindCheckContainers(logger lager.Logger, pipelineName string, resourceName string, variablesFactory creds.VariablesFactory) ([]Container, map[int]time.Time, error) {
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

	variables := variablesFactory.NewVariables(t.name, pipeline.Name())

	versionedResourceTypes := pipelineResourceTypes.Deserialize()

	source, err := creds.NewSource(variables, resource.Source()).Evaluate()
	if err != nil {
		return nil, nil, err
	}

	resourceConfigFactory := NewResourceConfigFactory(t.conn, t.lockFactory)
	resourceConfig, err := resourceConfigFactory.FindOrCreateResourceConfig(
		logger,
		resource.Type(),
		source,
		creds.NewVersionedResourceTypes(variables, versionedResourceTypes),
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

func (t *team) saveJob(tx Tx, job atc.JobConfig, pipelineID int, groups []string) error {
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
		SET config = $3, interruptible = $4, active = true, nonce = $5, tags = $6
		WHERE name = $1 AND pipeline_id = $2
	`, job.Name, pipelineID, encryptedPayload, job.Interruptible, nonce, pq.Array(groups))
	if err != nil {
		return err
	}

	if updated {
		return nil
	}

	_, err = tx.Exec(`
		INSERT INTO jobs (name, pipeline_id, config, interruptible, active, nonce, tags)
		VALUES ($1, $2, $3, $4, true, $5, $6)
	`, job.Name, pipelineID, encryptedPayload, job.Interruptible, nonce, pq.Array(groups))

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

	updated, err := checkIfRowsUpdated(tx, `
		UPDATE resources
		SET config = $3, active = true, nonce = $4
		WHERE name = $1 AND pipeline_id = $2
	`, resource.Name, pipelineID, encryptedPayload, nonce)
	if err != nil {
		return err
	}

	if resource.Version != nil {
		resourceIDQuery := `
				resource_pins.resource_id =
					(SELECT id FROM resources WHERE name = ? AND pipeline_id = ?)`

		_, err = psql.Delete("resource_pins").
			Where(resourceIDQuery, resource.Name, pipelineID).
			RunWith(tx).
			Exec()

		if err != nil {
			return err
		}
	}

	if updated {
		return nil
	}

	_, err = tx.Exec(`
		INSERT INTO resources (name, pipeline_id, config, active, nonce)
		VALUES ($1, $2, $3, true, $4)
	`, resource.Name, pipelineID, encryptedPayload, nonce)

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
	var groups sql.NullString
	err := scan.Scan(&p.id, &p.name, &groups, &p.configVersion, &p.teamID, &p.teamName, &p.paused, &p.public)
	if err != nil {
		return err
	}

	if groups.Valid {
		var pipelineGroups atc.GroupConfigs
		err = json.Unmarshal([]byte(groups.String), &pipelineGroups)
		if err != nil {
			return err
		}

		p.groups = pipelineGroups
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
