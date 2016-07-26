package db

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/concourse/atc"
)

//go:generate counterfeiter . TeamDB

type TeamDB interface {
	GetPipelines() ([]SavedPipeline, error)
	GetPipelineByName(pipelineName string) (SavedPipeline, error)

	OrderPipelines([]string) error

	GetTeam() (SavedTeam, bool, error)
	UpdateBasicAuth(basicAuth *BasicAuth) (SavedTeam, error)
	UpdateGitHubAuth(gitHubAuth *GitHubAuth) (SavedTeam, error)
	UpdateUAAAuth(uaaAuth *UAAAuth) (SavedTeam, error)

	GetConfig(pipelineName string) (atc.Config, atc.RawConfig, ConfigVersion, error)
	SaveConfig(string, atc.Config, ConfigVersion, PipelinePausedState) (SavedPipeline, bool, error)

	CreateOneOffBuild() (Build, error)
	GetBuilds(page Page, publicOnly bool) ([]Build, Pagination, error)
	GetBuild(buildID int) (Build, bool, error)

	Workers() ([]SavedWorker, error)
	FindContainersByDescriptors(id Container) ([]SavedContainer, error)
}

type teamDB struct {
	teamName string

	buildPrepHelper buildPreparationHelper
	conn            Conn
	buildFactory    *buildFactory
}

func (db *teamDB) GetPipelineByName(pipelineName string) (SavedPipeline, error) {
	row := db.conn.QueryRow(`
		SELECT `+pipelineColumns+`
		FROM pipelines p
		INNER JOIN teams t ON t.id = p.team_id
		WHERE p.name = $1
		AND p.team_id = (
			SELECT id FROM teams WHERE name = $2
		)
	`, pipelineName, db.teamName)

	return scanPipeline(row)
}

func (db *teamDB) GetPipelines() ([]SavedPipeline, error) {
	rows, err := db.conn.Query(`
		SELECT `+pipelineColumns+`
		FROM pipelines p
		INNER JOIN teams t ON t.id = p.team_id
		WHERE team_id = (
			SELECT id FROM teams WHERE name = $1
		) OR public = true
		ORDER BY ordering
	`, db.teamName)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

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

func (db *teamDB) OrderPipelines(pipelineNames []string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	var pipelineCount int

	var teamID int
	err = tx.QueryRow(`SELECT id FROM teams WHERE name = $1`, db.teamName).Scan(&teamID)
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
			WHERE name = $2
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
	err = tx.QueryRow(`SELECT id FROM teams WHERE name = $1`, db.teamName).Scan(&teamID)
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
		err = db.registerResource(tx, resource.Name, savedPipeline.ID)
		if err != nil {
			return SavedPipeline{}, false, err
		}
	}

	for _, resourceType := range config.ResourceTypes {
		err = db.registerResourceType(tx, resourceType, savedPipeline.ID)
		if err != nil {
			return SavedPipeline{}, false, err
		}
	}

	for _, job := range config.Jobs {
		err = db.registerJob(tx, job.Name, savedPipeline.ID)
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

func (db *teamDB) registerJob(tx Tx, name string, pipelineID int) error {
	_, err := tx.Exec(`
		INSERT INTO jobs (name, pipeline_id)
		SELECT $1, $2
		WHERE NOT EXISTS (
			SELECT 1 FROM jobs WHERE name = $1 AND pipeline_id = $2
		)
	`, name, pipelineID)

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

func (db *teamDB) registerResource(tx Tx, name string, pipelineID int) error {
	_, err := tx.Exec(`
		INSERT INTO resources (name, pipeline_id)
		SELECT $1, $2
		WHERE NOT EXISTS (
			SELECT 1 FROM resources WHERE name = $1 AND pipeline_id = $2
		)
	`, name, pipelineID)

	return swallowUniqueViolation(err)
}

func (db *teamDB) registerResourceType(tx Tx, resourceType atc.ResourceType, pipelineID int) error {
	_, err := tx.Exec(`
		INSERT INTO resource_types (name, type, pipeline_id)
		SELECT $1, $2, $3
		WHERE NOT EXISTS (
			SELECT 1 FROM resource_types
				WHERE name = $1
				AND type = $2
				AND pipeline_id = $3
		)
	`, resourceType.Name, resourceType.Type, pipelineID)

	return swallowUniqueViolation(err)
}

func (db *teamDB) GetTeam() (SavedTeam, bool, error) {
	query := fmt.Sprintf(`
		SELECT id, name, admin, basic_auth, github_auth, uaa_auth
		FROM teams
		WHERE name ILIKE '%s'
	`, db.teamName,
	)
	savedTeam, err := db.queryTeam(query)
	if err != nil {
		if err == sql.ErrNoRows {
			return savedTeam, false, nil
		}

		return savedTeam, false, err
	}

	return savedTeam, true, nil
}

func (db *teamDB) queryTeam(query string) (SavedTeam, error) {
	var basicAuth, gitHubAuth, uaaAuth sql.NullString
	var savedTeam SavedTeam

	tx, err := db.conn.Begin()
	if err != nil {
		return SavedTeam{}, err
	}
	defer tx.Rollback()

	err = tx.QueryRow(query).Scan(
		&savedTeam.ID,
		&savedTeam.Name,
		&savedTeam.Admin,
		&basicAuth,
		&gitHubAuth,
		&uaaAuth,
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

	return savedTeam, nil
}

func (db *teamDB) UpdateBasicAuth(basicAuth *BasicAuth) (SavedTeam, error) {
	encryptedBasicAuth, err := basicAuth.EncryptedJSON()
	if err != nil {
		return SavedTeam{}, err
	}

	query := fmt.Sprintf(`
		UPDATE teams
		SET basic_auth = '%s'
		WHERE name ILIKE '%s'
		RETURNING id, name, admin, basic_auth, github_auth, uaa_auth
	`, encryptedBasicAuth, db.teamName)
	return db.queryTeam(query)
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

	return db.queryTeam(fmt.Sprintf(`
		UPDATE teams
		SET github_auth = '%s'
		WHERE name ILIKE '%s'
		RETURNING id, name, admin, basic_auth, github_auth, uaa_auth
	`, string(jsonEncodedGitHubAuth), db.teamName,
	))
}

func (db *teamDB) UpdateUAAAuth(uaaAuth *UAAAuth) (SavedTeam, error) {
	jsonEncodedUAAAuth, err := json.Marshal(uaaAuth)
	if err != nil {
		return SavedTeam{}, err
	}

	return db.queryTeam(fmt.Sprintf(`
		UPDATE teams
		SET uaa_auth = '%s'
		WHERE name ILIKE '%s'
		RETURNING id, name, admin, basic_auth, github_auth, uaa_auth
	`, string(jsonEncodedUAAAuth), db.teamName,
	))
}

func (db *teamDB) CreateOneOffBuild() (Build, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	var teamID int
	err = tx.QueryRow(`SELECT id FROM teams WHERE name = $1`, db.teamName).Scan(&teamID)
	if err != nil {
		return nil, err
	}

	build, _, err := db.buildFactory.ScanBuild(tx.QueryRow(`
		INSERT INTO builds (name, team_id, status)
		VALUES (nextval('one_off_name'), $1, 'pending')
		RETURNING `+buildColumns+`, null, null, null, $2::text
	`, teamID, string(db.teamName)))

	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(fmt.Sprintf(`
		CREATE SEQUENCE %s MINVALUE 0
	`, buildEventSeq(build.ID())))
	if err != nil {
		return nil, err
	}

	err = db.buildPrepHelper.CreateBuildPreparation(tx, build.ID())
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
	err := reapExpiredWorkers(db.conn)
	if err != nil {
		return nil, err
	}

	rows, err := db.conn.Query(`
		SELECT `+workerColumns+`
		FROM workers as w
		LEFT OUTER JOIN teams as t ON t.id = w.team_id
		WHERE t.name = $1 OR w.team_id IS NULL
	`, db.teamName)
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

func (db *teamDB) GetBuilds(page Page, publicOnly bool) ([]Build, Pagination, error) {
	query := `
		SELECT ` + qualifiedBuildColumns + `
		FROM builds b
		LEFT OUTER JOIN jobs j ON b.job_id = j.id
		LEFT OUTER JOIN pipelines p ON j.pipeline_id = p.id
		LEFT OUTER JOIN teams t ON b.team_id = t.id
		WHERE p.public = true
	`
	if !publicOnly {
		query = query + ` OR t.name = $1`
	} else {
		query = query + ` AND $1 = $1`
	}

	var rows *sql.Rows
	var err error

	if page.Since == 0 && page.Until == 0 {
		rows, err = db.conn.Query(fmt.Sprintf(`
			%s
			ORDER BY b.id DESC
			LIMIT $2
		`, query), db.teamName, page.Limit)
	} else if page.Until != 0 {
		rows, err = db.conn.Query(fmt.Sprintf(`
			SELECT sub.*
				FROM (
						%s
				AND b.id > $2
				ORDER BY b.id ASC
				LIMIT $3
			) sub
			ORDER BY sub.id DESC
		`, query), db.teamName, page.Until, page.Limit)
	} else {
		rows, err = db.conn.Query(fmt.Sprintf(`
			%s
			AND b.id < $2
			ORDER BY b.id DESC
			LIMIT $3
		`, query), db.teamName, page.Since, page.Limit)
	}

	if err != nil {
		return nil, Pagination{}, err
	}

	defer rows.Close()

	builds := []Build{}

	for rows.Next() {
		build, _, err := db.buildFactory.ScanBuild(rows)
		if err != nil {
			return nil, Pagination{}, err
		}

		builds = append(builds, build)
	}

	if len(builds) == 0 {
		return builds, Pagination{}, nil
	}

	var minID int
	var maxID int

	maxMinQuery := `
		SELECT COALESCE(MAX(b.id), 0) as maxID,
		COALESCE(MIN(b.id), 0) as minID
		FROM builds b
	`

	err = db.conn.QueryRow(maxMinQuery).Scan(&maxID, &minID)
	if err != nil {
		return nil, Pagination{}, err
	}

	first := builds[0]
	last := builds[len(builds)-1]

	var pagination Pagination

	if first.ID() < maxID {
		pagination.Previous = &Page{
			Until: first.ID(),
			Limit: page.Limit,
		}
	}

	if last.ID() > minID {
		pagination.Next = &Page{
			Since: last.ID(),
			Limit: page.Limit,
		}
	}

	return builds, pagination, nil
}

func (db *teamDB) GetBuild(buildID int) (Build, bool, error) {
	return db.buildFactory.ScanBuild(db.conn.QueryRow(`
		SELECT `+qualifiedBuildColumns+`
		FROM builds b
		LEFT OUTER JOIN jobs j ON b.job_id = j.id
		LEFT OUTER JOIN pipelines p ON j.pipeline_id = p.id
		LEFT OUTER JOIN teams t ON b.team_id = t.id
		WHERE b.id = $1
		AND (t.name = $2 OR p.public = true)
	`, buildID, db.teamName))
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
