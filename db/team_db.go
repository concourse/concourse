package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	sq "github.com/Masterminds/squirrel"

	"github.com/concourse/atc"
)

//go:generate counterfeiter . TeamDB

type TeamDB interface {
	GetPipelines() ([]SavedPipeline, error)
	GetPublicPipelines() ([]SavedPipeline, error)
	GetPrivateAndAllPublicPipelines() ([]SavedPipeline, error)

	GetPipelineByName(pipelineName string) (SavedPipeline, bool, error)

	OrderPipelines([]string) error

	GetTeam() (SavedTeam, bool, error)
	UpdateBasicAuth(basicAuth *BasicAuth) (SavedTeam, error)
	UpdateGitHubAuth(gitHubAuth *GitHubAuth) (SavedTeam, error)
	UpdateUAAAuth(uaaAuth *UAAAuth) (SavedTeam, error)
	UpdateGenericOAuth(genericOAuth *GenericOAuth) (SavedTeam, error)

	GetConfig(pipelineName string) (atc.Config, atc.RawConfig, ConfigVersion, error)
	SaveConfig(string, atc.Config, ConfigVersion, PipelinePausedState) (SavedPipeline, bool, error)

	CreateOneOffBuild() (Build, error)
	GetPrivateAndPublicBuilds(page Page) ([]Build, Pagination, error)

	Workers() ([]SavedWorker, error)
	GetContainer(handle string) (SavedContainer, bool, error)
	FindContainersByDescriptors(id Container) ([]SavedContainer, error)

	GetVolumes() ([]SavedVolume, error)
}

type teamDB struct {
	teamName string

	conn         Conn
	buildFactory *buildFactory
}

func (db *teamDB) GetPipelineByName(pipelineName string) (SavedPipeline, bool, error) {
	row := db.conn.QueryRow(`
		SELECT `+pipelineColumns+`
		FROM pipelines p
		INNER JOIN teams t ON t.id = p.team_id
		WHERE p.name = $1
		AND p.team_id = (
			SELECT id FROM teams WHERE LOWER(name) = LOWER($2)
		)
	`, pipelineName, db.teamName)
	pipeline, err := scanPipeline(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return SavedPipeline{}, false, nil
		}

		return SavedPipeline{}, false, err
	}

	return pipeline, true, nil
}

func (db *teamDB) GetPipelines() ([]SavedPipeline, error) {
	rows, err := db.conn.Query(`
		SELECT `+pipelineColumns+`
		FROM pipelines p
		INNER JOIN teams t ON t.id = p.team_id
		WHERE team_id = (
			SELECT id FROM teams WHERE LOWER(name) = LOWER($1)
		)
		ORDER BY ordering
	`, db.teamName)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	return scanPipelines(rows)
}

func (db *teamDB) GetPublicPipelines() ([]SavedPipeline, error) {
	rows, err := db.conn.Query(`
		SELECT `+pipelineColumns+`
		FROM pipelines p
		INNER JOIN teams t ON t.id = p.team_id
		WHERE team_id = (
			SELECT id FROM teams WHERE LOWER(name) = LOWER($1)
		)
		AND public = true
		ORDER BY ordering
	`, db.teamName)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	return scanPipelines(rows)
}

func (db *teamDB) GetPrivateAndAllPublicPipelines() ([]SavedPipeline, error) {
	rows, err := db.conn.Query(`
		SELECT `+pipelineColumns+`
		FROM pipelines p
		INNER JOIN teams t ON t.id = p.team_id
		WHERE team_id = (SELECT id FROM teams WHERE LOWER(name) = LOWER($1))
		ORDER BY ordering
	`, db.teamName)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	currentTeamPipelines, err := scanPipelines(rows)
	if err != nil {
		return nil, err
	}

	otherRows, err := db.conn.Query(`
		SELECT `+pipelineColumns+`
		FROM pipelines p
		INNER JOIN teams t ON t.id = p.team_id
		WHERE team_id != (SELECT id FROM teams WHERE LOWER(name) = LOWER($1))
		AND public = true
		ORDER BY team_name, ordering
	`, db.teamName)
	if err != nil {
		return nil, err
	}

	defer otherRows.Close()

	otherTeamPipelines, err := scanPipelines(otherRows)
	if err != nil {
		return nil, err
	}

	return append(currentTeamPipelines, otherTeamPipelines...), nil
}

func (db *teamDB) OrderPipelines(pipelineNames []string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	var pipelineCount int

	var teamID int
	err = tx.QueryRow(`SELECT id FROM teams WHERE LOWER(name) = LOWER($1)`, db.teamName).Scan(&teamID)
	if err != nil {
		return err
	}

	err = tx.QueryRow(`
		SELECT COUNT(1)
		FROM pipelines
		WHERE team_id = $1
	`, teamID).Scan(&pipelineCount)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE pipelines
		SET ordering = $1
		WHERE team_id = $2
	`, pipelineCount+1, teamID)

	if err != nil {
		return err
	}

	for i, name := range pipelineNames {
		_, err = tx.Exec(`
			UPDATE pipelines
			SET ordering = $1
			WHERE name = $2
			AND team_id = $3
		`, i, name, teamID)

		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (db *teamDB) GetConfig(pipelineName string) (atc.Config, atc.RawConfig, ConfigVersion, error) {
	var configBlob []byte
	var version int
	err := db.conn.QueryRow(`
		SELECT config, version
		FROM pipelines
		WHERE name = $1 AND team_id = (
			SELECT id
			FROM teams
			WHERE LOWER(name) = LOWER($2)
		)
	`, pipelineName, db.teamName).Scan(&configBlob, &version)
	if err != nil {
		if err == sql.ErrNoRows {
			return atc.Config{}, atc.RawConfig(""), 0, nil
		}
		return atc.Config{}, atc.RawConfig(""), 0, err
	}

	var config atc.Config
	err = json.Unmarshal(configBlob, &config)
	if err != nil {
		return atc.Config{}, atc.RawConfig(string(configBlob)), ConfigVersion(version), atc.MalformedConfigError{err}
	}

	return config, atc.RawConfig(string(configBlob)), ConfigVersion(version), nil
}

func (db *teamDB) SaveConfig(
	pipelineName string,
	config atc.Config,
	from ConfigVersion,
	pausedState PipelinePausedState,
) (SavedPipeline, bool, error) {
	payload, err := json.Marshal(config)
	if err != nil {
		return SavedPipeline{}, false, err
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return SavedPipeline{}, false, err
	}

	defer tx.Rollback()

	var teamID int
	err = tx.QueryRow(`SELECT id FROM teams WHERE LOWER(name) = LOWER($1)`, db.teamName).Scan(&teamID)
	if err != nil {
		return SavedPipeline{}, false, err
	}

	var created bool
	var savedPipeline SavedPipeline

	var existingConfig int
	err = tx.QueryRow(`
		SELECT COUNT(1)
		FROM pipelines
		WHERE name = $1
	  AND team_id = $2
	`, pipelineName, teamID).Scan(&existingConfig)
	if err != nil {
		return SavedPipeline{}, false, err
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
		`, pipelineName, payload, pausedState.Bool(), teamID))
		if err != nil {
			return SavedPipeline{}, false, err
		}

		created = true

		_, err = tx.Exec(fmt.Sprintf(`
		CREATE TABLE pipeline_build_events_%[1]d ()
		INHERITS (build_events);
		`, savedPipeline.ID))
		if err != nil {
			return SavedPipeline{}, false, err
		}

		_, err = tx.Exec(fmt.Sprintf(`
		CREATE INDEX pipeline_build_events_%[1]d_build_id ON pipeline_build_events_%[1]d (build_id);
		`, savedPipeline.ID))
		if err != nil {
			return SavedPipeline{}, false, err
		}

		_, err = tx.Exec(fmt.Sprintf(`
		CREATE UNIQUE INDEX pipeline_build_events_%[1]d_build_id_event_id ON pipeline_build_events_%[1]d (build_id, event_id);
		`, savedPipeline.ID))
		if err != nil {
			return SavedPipeline{}, false, err
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
			`, payload, pipelineName, from, teamID))
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
			`, payload, pausedState.Bool(), pipelineName, from, teamID))
		}

		if err != nil && err != sql.ErrNoRows {
			return SavedPipeline{}, false, err
		}

		if savedPipeline.ID == 0 {
			return SavedPipeline{}, false, ErrConfigComparisonFailed
		}

		_, err = tx.Exec(`
      DELETE FROM jobs_serial_groups
      WHERE job_id in (
        SELECT j.id
        FROM jobs j
        WHERE j.pipeline_id = $1
      )
		`, savedPipeline.ID)
		if err != nil {
			return SavedPipeline{}, false, err
		}
	}

	for _, resource := range config.Resources {
		err = db.saveResource(tx, resource, savedPipeline.ID)
		if err != nil {
			return SavedPipeline{}, false, err
		}
	}

	for _, resourceType := range config.ResourceTypes {
		err = db.saveResourceType(tx, resourceType, savedPipeline.ID)
		if err != nil {
			return SavedPipeline{}, false, err
		}
	}

	for _, job := range config.Jobs {
		err = db.saveJob(tx, job, savedPipeline.ID)
		if err != nil {
			return SavedPipeline{}, false, err
		}

		for _, sg := range job.SerialGroups {
			err = db.registerSerialGroup(tx, job.Name, sg, savedPipeline.ID)
			if err != nil {
				return SavedPipeline{}, false, err
			}
		}
	}

	return savedPipeline, created, tx.Commit()
}

func (db *teamDB) saveJob(tx Tx, job atc.JobConfig, pipelineID int) error {
	configPayload, err := json.Marshal(job)
	if err != nil {
		return err
	}

	updated, err := checkIfRowsUpdated(tx, `
		UPDATE jobs
		SET config = $3
		WHERE name = $1 AND pipeline_id = $2
	`, job.Name, pipelineID, configPayload)
	if err != nil {
		return err
	}

	if updated {
		return nil
	}

	_, err = tx.Exec(`
		INSERT INTO jobs (name, pipeline_id, config)
		VALUES ($1, $2, $3)
	`, job.Name, pipelineID, configPayload)

	return swallowUniqueViolation(err)
}

func (db *teamDB) registerSerialGroup(tx Tx, jobName, serialGroup string, pipelineID int) error {
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

func (db *teamDB) saveResource(tx Tx, resource atc.ResourceConfig, pipelineID int) error {
	configPayload, err := json.Marshal(resource)
	if err != nil {
		return err
	}

	updated, err := checkIfRowsUpdated(tx, `
		UPDATE resources
		SET config = $3
		WHERE name = $1 AND pipeline_id = $2
	`, resource.Name, pipelineID, configPayload)
	if err != nil {
		return err
	}

	if updated {
		return nil
	}

	_, err = tx.Exec(`
		INSERT INTO resources (name, pipeline_id, config)
		VALUES ($1, $2, $3)
	`, resource.Name, pipelineID, configPayload)

	return swallowUniqueViolation(err)
}

func (db *teamDB) saveResourceType(tx Tx, resourceType atc.ResourceType, pipelineID int) error {
	configPayload, err := json.Marshal(resourceType)
	if err != nil {
		return err
	}

	updated, err := checkIfRowsUpdated(tx, `
		UPDATE resource_types
		SET config = $3,
			type = $4
		WHERE name = $1 AND pipeline_id = $2
	`, resourceType.Name, pipelineID, configPayload, resourceType.Type)
	if err != nil {
		return err
	}

	if updated {
		return nil
	}

	_, err = tx.Exec(`
		INSERT INTO resource_types (name, type, pipeline_id, config)
		VALUES ($1, $2, $3, $4)
	`, resourceType.Name, resourceType.Type, pipelineID, configPayload)

	return swallowUniqueViolation(err)
}

func (db *teamDB) GetTeam() (SavedTeam, bool, error) {
	query := `
		SELECT id, name, admin, basic_auth, github_auth, uaa_auth, genericoauth_auth
		FROM teams
		WHERE LOWER(name) = LOWER($1)
	`
	params := []interface{}{db.teamName}
	savedTeam, err := db.queryTeam(query, params)
	if err != nil {
		if err == sql.ErrNoRows {
			return savedTeam, false, nil
		}

		return savedTeam, false, err
	}

	return savedTeam, true, nil
}

func (db *teamDB) queryTeam(query string, params []interface{}) (SavedTeam, error) {
	var basicAuth, gitHubAuth, uaaAuth, genericOAuth sql.NullString
	var savedTeam SavedTeam

	tx, err := db.conn.Begin()
	if err != nil {
		return SavedTeam{}, err
	}
	defer tx.Rollback()

	err = tx.QueryRow(query, params...).Scan(
		&savedTeam.ID,
		&savedTeam.Name,
		&savedTeam.Admin,
		&basicAuth,
		&gitHubAuth,
		&uaaAuth,
		&genericOAuth,
	)
	if err != nil {
		return savedTeam, err
	}
	err = tx.Commit()
	if err != nil {
		return savedTeam, err
	}

	if basicAuth.Valid {
		err = json.Unmarshal([]byte(basicAuth.String), &savedTeam.BasicAuth)
		if err != nil {
			return savedTeam, err
		}
	}

	if gitHubAuth.Valid {
		err = json.Unmarshal([]byte(gitHubAuth.String), &savedTeam.GitHubAuth)
		if err != nil {
			return savedTeam, err
		}
	}

	if uaaAuth.Valid {
		err = json.Unmarshal([]byte(uaaAuth.String), &savedTeam.UAAAuth)
		if err != nil {
			return savedTeam, err
		}
	}

	if genericOAuth.Valid {
		err = json.Unmarshal([]byte(genericOAuth.String), &savedTeam.GenericOAuth)
		if err != nil {
			return savedTeam, err
		}
	}

	return savedTeam, nil
}

func (db *teamDB) UpdateBasicAuth(basicAuth *BasicAuth) (SavedTeam, error) {
	encryptedBasicAuth, err := basicAuth.EncryptedJSON()
	if err != nil {
		return SavedTeam{}, err
	}

	query := `
		UPDATE teams
		SET basic_auth = $1
		WHERE LOWER(name) = LOWER($2)
		RETURNING id, name, admin, basic_auth, github_auth, uaa_auth, genericoauth_auth
	`

	params := []interface{}{encryptedBasicAuth, db.teamName}

	return db.queryTeam(query, params)
}

func (db *teamDB) UpdateGitHubAuth(gitHubAuth *GitHubAuth) (SavedTeam, error) {
	var auth *GitHubAuth
	if gitHubAuth != nil && gitHubAuth.ClientID != "" && gitHubAuth.ClientSecret != "" {
		auth = gitHubAuth
	}
	jsonEncodedGitHubAuth, err := json.Marshal(auth)
	if err != nil {
		return SavedTeam{}, err
	}

	query := `
		UPDATE teams
		SET github_auth = $1
		WHERE LOWER(name) = LOWER($2)
		RETURNING id, name, admin, basic_auth, github_auth, uaa_auth, genericoauth_auth
	`
	params := []interface{}{string(jsonEncodedGitHubAuth), db.teamName}
	return db.queryTeam(query, params)
}

func (db *teamDB) UpdateUAAAuth(uaaAuth *UAAAuth) (SavedTeam, error) {
	jsonEncodedUAAAuth, err := json.Marshal(uaaAuth)
	if err != nil {
		return SavedTeam{}, err
	}

	query := `
		UPDATE teams
		SET uaa_auth = $1
		WHERE LOWER(name) = LOWER($2)
		RETURNING id, name, admin, basic_auth, github_auth, uaa_auth, genericoauth_auth
	`
	params := []interface{}{string(jsonEncodedUAAAuth), db.teamName}
	return db.queryTeam(query, params)
}

func (db *teamDB) UpdateGenericOAuth(genericOAuth *GenericOAuth) (SavedTeam, error) {
	jsonEncodedGenericOAuth, err := json.Marshal(genericOAuth)
	if err != nil {
		return SavedTeam{}, err
	}

	query := `
		UPDATE teams
		SET genericoauth_auth = $1
		WHERE LOWER(name) = LOWER($2)
		RETURNING id, name, admin, basic_auth, github_auth, uaa_auth, genericoauth_auth
	`
	params := []interface{}{string(jsonEncodedGenericOAuth), db.teamName}
	return db.queryTeam(query, params)
}

func (db *teamDB) CreateOneOffBuild() (Build, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	build, _, err := db.buildFactory.ScanBuild(tx.QueryRow(`
		INSERT INTO builds (name, team_id, status)
		SELECT nextval('one_off_name'), t.id, 'pending'
		FROM teams t WHERE LOWER(t.name) = LOWER($1)
		RETURNING `+buildColumns+`, null, null, null,
		(
			SELECT name FROM teams WHERE LOWER(name) = LOWER($1)
		)
	`, string(db.teamName)))
	if err != nil {
		return nil, err
	}

	err = createBuildEventSeq(tx, build.ID())
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return build, nil
}

func (db *teamDB) Workers() ([]SavedWorker, error) {
	team, found, err := db.GetTeam()
	if err != nil {
		return nil, err
	}

	var teamID int
	if found {
		teamID = team.ID
	}

	rows, err := db.conn.Query(`
		SELECT `+workerColumns+`
		FROM workers as w
		LEFT OUTER JOIN teams as t
			ON t.id = w.team_id
		WHERE (t.id = $1 OR w.team_id IS NULL)
		AND (expires IS NULL OR expires > NOW())
	`, teamID)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	savedWorkers := []SavedWorker{}
	for rows.Next() {
		savedWorker, err := scanWorker(rows, true)
		if err != nil {
			return nil, err
		}

		savedWorkers = append(savedWorkers, savedWorker)
	}

	return savedWorkers, nil
}

func (db *teamDB) GetPrivateAndPublicBuilds(page Page) ([]Build, Pagination, error) {
	buildsQuery := sq.Select(qualifiedBuildColumns).From("builds b").
		LeftJoin("jobs j ON b.job_id = j.id").
		LeftJoin("pipelines p ON j.pipeline_id = p.id").
		LeftJoin("teams t ON b.team_id = t.id").
		Where(sq.Or{sq.Eq{"p.public": true}, sq.Eq{"LOWER(t.name)": strings.ToLower(db.teamName)}})

	return getBuildsWithPagination(buildsQuery, page, db.conn, db.buildFactory)
}

func scanPipeline(rows scannable) (SavedPipeline, error) {
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
		return SavedPipeline{}, err
	}

	var config atc.Config
	err = json.Unmarshal(configBlob, &config)
	if err != nil {
		return SavedPipeline{}, err
	}

	return SavedPipeline{
		ID:       id,
		Paused:   paused,
		Public:   public,
		TeamID:   teamID,
		TeamName: teamName,
		Pipeline: Pipeline{
			Name:    name,
			Config:  config,
			Version: ConfigVersion(version),
		},
	}, nil
}

func scanPipelines(rows *sql.Rows) ([]SavedPipeline, error) {
	pipelines := []SavedPipeline{}

	for rows.Next() {
		pipeline, err := scanPipeline(rows)
		if err != nil {
			return nil, err
		}

		pipelines = append(pipelines, pipeline)
	}

	return pipelines, nil
}

type PipelinePausedState string

const (
	PipelinePaused   PipelinePausedState = "paused"
	PipelineUnpaused PipelinePausedState = "unpaused"
	PipelineNoChange PipelinePausedState = "nochange"
)

func (state PipelinePausedState) Bool() *bool {
	yes := true
	no := false

	switch state {
	case PipelinePaused:
		return &yes
	case PipelineUnpaused:
		return &no
	case PipelineNoChange:
		return nil
	default:
		panic("unknown pipeline state")
	}
}
