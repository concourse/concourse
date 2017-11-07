package migrations

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db/migration"
	internal_163 "github.com/concourse/atc/db/migrations/internal/163"
	internal_26 "github.com/concourse/atc/db/migrations/internal/26"
)

func AddReplicatedFromToVolumes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE volumes ADD COLUMN replicated_from text DEFAULT null;
	`)
	if err != nil {
		return err
	}

	return nil
}

func AddSizeToVolumes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE volumes
		ADD COLUMN size integer default 0;
`)
	return err
}

func AddFirstLoggedBuildIDToJobsAndReapTimeToBuildsAndLeases(tx migration.LimitedTx) error {
	_, err := tx.Exec(`ALTER TABLE jobs ADD COLUMN first_logged_build_id int NOT NULL DEFAULT 0`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`ALTER TABLE builds ADD COLUMN reap_time timestamp with time zone`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE TABLE leases (
    id serial PRIMARY KEY,
		name text NOT NULL,
		last_invalidated timestamp NOT NULL DEFAULT 'epoch',
    CONSTRAINT constraint_leases_name_unique UNIQUE (name)
	)`)

	return err
}

func AddMissingInputReasonsToBuildPreparation(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE build_preparation ADD COLUMN missing_input_reasons json DEFAULT '{}';
	`)
	if err != nil {
		return err
	}

	return nil
}

func MakeVolumeSizeBigint(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE volumes ALTER COLUMN size TYPE bigint;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE volumes RENAME COLUMN size TO size_in_bytes;
	`)
	return err
}

func MakeContainersExpiresAtNullable(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers
		ALTER COLUMN expires_at DROP NOT NULL;
	`)
	return err
}

func AddContainerIDToVolumes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers ADD COLUMN id serial PRIMARY KEY;
	`)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE volumes ADD COLUMN container_id int;
	`)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`ALTER TABLE volumes ADD CONSTRAINT fkey_container_id FOREIGN KEY (container_id) REFERENCES containers (id);`)

	return err
}

func AddBuildEvents(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    CREATE TABLE build_events (
      id serial PRIMARY KEY,
      build_id integer REFERENCES builds (id),
			type varchar(32) NOT NULL,
      payload text NOT NULL
    )
  `)
	if err != nil {
		return err
	}

	cursor := 0

	for {
		var id int
		var buildLog sql.NullString

		err := tx.QueryRow(`
			SELECT id, log
			FROM builds
			WHERE id > $1
			ORDER BY id ASC
			LIMIT 1
		`, cursor).Scan(&id, &buildLog)
		if err != nil {
			if err == sql.ErrNoRows {
				break
			}

			return err
		}

		cursor = id

		if !buildLog.Valid {
			continue
		}

		logBuf := bytes.NewBufferString(buildLog.String)
		decoder := json.NewDecoder(logBuf)

		for {
			var entry logEntry

			err := decoder.Decode(&entry)
			if err != nil {
				if err != io.EOF {
					// non-JSON log; assume v0.0

					_, err = tx.Exec(`
						INSERT INTO build_events (build_id, type, payload)
						VALUES ($1, $2, $3)
					`, id, "version", "0.0")
					if err != nil {
						return err
					}

					_, err = tx.Exec(`
							INSERT INTO build_events (build_id, type, payload)
							VALUES ($1, $2, $3)
						`, id, "log", buildLog.String)
					if err != nil {
						return err
					}
				}

				break
			}

			if entry.Type != "" && entry.EventPayload != nil {
				_, err = tx.Exec(`
						INSERT INTO build_events (build_id, type, payload)
						VALUES ($1, $2, $3)
					`, id, entry.Type, []byte(*entry.EventPayload))
				if err != nil {
					return err
				}

				continue
			}

			if entry.Version != "" {
				versionEnc, err := json.Marshal(entry.Version)
				if err != nil {
					return err
				}

				_, err = tx.Exec(`
					INSERT INTO build_events (build_id, type, payload)
					VALUES ($1, $2, $3)
				`, id, "version", versionEnc)
				if err != nil {
					return err
				}

				continue
			}

			return fmt.Errorf("malformed event stream; got stuck at %s", logBuf.String())
		}
	}

	_, err = tx.Exec(`
		ALTER TABLE builds
		DROP COLUMN log
	`)
	if err != nil {
		return err
	}

	return nil
}

type logEntry struct {
	// either an event...
	Type         string           `json:"type"`
	EventPayload *json.RawMessage `json:"event"`

	// ...or a version
	Version string `json:"version"`
}

func AddOnDeleteSetNullToFKeyContainerId(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE volumes DROP CONSTRAINT fkey_container_id;
	`)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`ALTER TABLE volumes ADD CONSTRAINT fkey_container_id FOREIGN KEY (container_id) REFERENCES containers (id) ON DELETE SET NULL;`)

	return err
}

func AddUAAAuthToTeams(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE teams
    ADD COLUMN uaa_auth json null;
	`)
	return err
}

func AddTeamIDToBuilds(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE builds
    ADD COLUMN team_id integer REFERENCES teams (id);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE builds
		SET team_id = sub.id
		FROM (
			SELECT id
			FROM teams
			WHERE name = 'main'
		) AS sub
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE builds ALTER COLUMN team_id SET NOT NULL;
	`)
	return err
}

func AddPublicToPipelines(tx migration.LimitedTx) error {
	_, err := tx.Exec(`ALTER TABLE pipelines ADD COLUMN public boolean NOT NULL default false`)
	return err
}

func AddTeamIDToWorkers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE workers
      ADD COLUMN team_id integer
			REFERENCES teams (id)
			ON DELETE CASCADE;
	`)

	return err
}

func AddTeamIDToContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE containers
      ADD COLUMN team_id integer
			REFERENCES teams (id)
			ON DELETE SET NULL;
	`)

	return err
}

func AddTeamIDToVolumes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE volumes
      ADD COLUMN team_id integer
      REFERENCES teams (id)
      ON DELETE SET NULL;
	`)

	return err
}

func AddNextBuildInputs(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		CREATE TABLE independent_build_inputs (
			id serial PRIMARY KEY,
			job_id integer NOT NULL,
			CONSTRAINT independent_build_inputs_job_id_fkey
				FOREIGN KEY (job_id)
				REFERENCES jobs (id)
				ON DELETE CASCADE,
			input_name text NOT NULL,
			CONSTRAINT independent_build_inputs_unique_job_id_input_name
				UNIQUE (job_id, input_name),
			version_id integer NOT NULL,
			CONSTRAINT independent_build_inputs_version_id_fkey
				FOREIGN KEY (version_id)
				REFERENCES versioned_resources (id)
				ON DELETE CASCADE,
			first_occurrence bool NOT NULL
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE TABLE next_build_inputs (
			id serial PRIMARY KEY,
			job_id integer NOT NULL,
			CONSTRAINT next_build_inputs_job_id_fkey
				FOREIGN KEY (job_id)
				REFERENCES jobs (id)
				ON DELETE CASCADE,
			input_name text NOT NULL,
			CONSTRAINT next_build_inputs_unique_job_id_input_name
				UNIQUE (job_id, input_name),
			version_id integer NOT NULL,
			CONSTRAINT next_build_inputs_version_id_fkey
				FOREIGN KEY (version_id)
				REFERENCES versioned_resources (id)
				ON DELETE CASCADE,
			first_occurrence bool NOT NULL
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE jobs
			ADD COLUMN resource_check_finished_at timestamp NOT NULL DEFAULT 'epoch',
			ADD COLUMN resource_check_waiver_end integer NOT NULL DEFAULT 0,
			ADD COLUMN inputs_determined bool NOT NULL DEFAULT false,
			ADD COLUMN max_in_flight_reached bool NOT NULL DEFAULT false
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE builds
			DROP COLUMN inputs_determined,
			DROP COLUMN last_scheduled
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		DROP TABLE build_preparation
	`)
	return err
}

func AddCaseInsenstiveUniqueIndexToTeamsName(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
ALTER TABLE teams
DROP CONSTRAINT constraint_teams_name_unique
    `)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`
	CREATE UNIQUE INDEX index_teams_name_unique_case_insensitive ON
	teams ( LOWER (name) )
	`)

	return err
}

func AddNonEmptyConstraintToTeamName(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
ALTER TABLE teams
ADD CONSTRAINT constraint_teams_name_not_empty CHECK(length(name)>0)
    `)

	if err != nil {
		return err
	}

	return err
}

func ReplaceBuildsAbortHijackURLsWithGuidAndEndpoint(tx migration.LimitedTx) error {
	_, err := tx.Exec(`ALTER TABLE builds ADD COLUMN guid varchar(36)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`ALTER TABLE builds ADD COLUMN endpoint varchar(128)`)
	if err != nil {
		return err
	}

	cursor := 0

	for {
		var id int
		var abortURLStr sql.NullString

		err := tx.QueryRow(`
			SELECT id, abort_url
			FROM builds
			WHERE id > $1
			LIMIT 1
		`, cursor).Scan(&id, &abortURLStr)
		if err != nil {
			if err == sql.ErrNoRows {
				break
			}

			return err
		}

		cursor = id

		if !abortURLStr.Valid {
			continue
		}

		// determine guid + endpoint from abort url
		//
		// format should be http://foo.com:5050/builds/some-guid/abort
		//
		// best-effort; skip if not possible, not a big deal

		abortURL, err := url.Parse(abortURLStr.String)
		if err != nil {
			continue
		}

		pathSegments := strings.Split(abortURL.Path, "/")
		if len(pathSegments) != 4 {
			continue
		}

		guid := pathSegments[2]
		endpoint := abortURL.Scheme + "://" + abortURL.Host

		_, err = tx.Exec(`
			UPDATE builds
			SET guid = $1, endpoint = $2
			WHERE id = $3
		`, guid, endpoint, id)
		if err != nil {
			continue
		}
	}

	_, err = tx.Exec(`ALTER TABLE builds DROP COLUMN abort_url`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`ALTER TABLE builds DROP COLUMN hijack_url`)
	if err != nil {
		return err
	}

	return nil
}

func AddGenericOAuthToTeams(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE teams
    ADD COLUMN genericoauth_auth json null;
	`)
	return err
}

func MigrateFromLeasesToLocks(tx migration.LimitedTx) error {
	_, err := tx.Exec(`DROP TABLE leases`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE jobs
		DROP COLUMN resource_check_finished_at,
		ADD COLUMN resource_checking bool NOT NULL DEFAULT false
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE resources DROP COLUMN checking
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE resource_types
		DROP COLUMN checking
	`)
	if err != nil {
		return err
	}

	return nil
}

func AddTeamNameToPipe(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE pipes
    ADD COLUMN team_id integer REFERENCES teams (id);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE pipes
		SET team_id = sub.id
		FROM (
			SELECT id
			FROM teams
			WHERE name = 'main'
		) AS sub
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE pipes ALTER COLUMN team_id SET NOT NULL;
	`)
	return err
}

func AddConfigToJobsResources(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE resources
		ADD COLUMN config json NOT NULL DEFAULT '{}',
		ADD COLUMN active bool NOT NULL DEFAULT false;
 	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE resource_types
		ADD COLUMN config json NOT NULL DEFAULT '{}',
		ADD COLUMN active bool NOT NULL DEFAULT false;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE jobs
		ADD COLUMN config json NOT NULL DEFAULT '{}',
		ADD COLUMN active bool NOT NULL DEFAULT false;
	`)
	if err != nil {
		return err
	}

	rows, err := tx.Query(`
    SELECT id, config
  	FROM pipelines
  `)
	if err != nil {
		return err
	}

	defer rows.Close()

	pipelineConfigs := map[int]atc.Config{}

	for rows.Next() {
		var pipelineID int
		var pipelineConfigPayload []byte
		err := rows.Scan(&pipelineID, &pipelineConfigPayload)
		if err != nil {
			return err
		}

		var pipelineConfig atc.Config
		err = json.Unmarshal(pipelineConfigPayload, &pipelineConfig)
		if err != nil {
			return err
		}

		pipelineConfigs[pipelineID] = pipelineConfig
	}

	for pipelineID, pipelineConfig := range pipelineConfigs {
		for _, jobConfig := range pipelineConfig.Jobs {
			jobConfigPayload, err := json.Marshal(jobConfig)
			if err != nil {
				return err
			}

			_, err = tx.Exec(`
				UPDATE jobs
				SET config = $1, active = true
				WHERE name = $2 AND pipeline_id = $3
			`, jobConfigPayload, jobConfig.Name, pipelineID)
			if err != nil {
				return err
			}

			for _, resourceConfig := range pipelineConfig.Resources {
				resourceConfigPayload, err := json.Marshal(resourceConfig)
				if err != nil {
					return err
				}

				_, err = tx.Exec(`
					UPDATE resources
					SET config = $1, active = true
					WHERE name = $2 AND pipeline_id = $3
			      `, resourceConfigPayload, resourceConfig.Name, pipelineID)
				if err != nil {
					return err
				}
			}

			for _, resourceTypeConfig := range pipelineConfig.ResourceTypes {
				resourceTypeConfigPayload, err := json.Marshal(resourceTypeConfig)
				if err != nil {
					return err
				}

				_, err = tx.Exec(`
					UPDATE resource_types
					SET config = $1, active = true
					WHERE name = $2 AND pipeline_id = $3
				 `, resourceTypeConfigPayload, resourceTypeConfig.Name, pipelineID)
				if err != nil {
					return err
				}
			}
		}
	}

	_, err = tx.Exec(`
		ALTER TABLE resources ALTER COLUMN config DROP DEFAULT;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE resource_types ALTER COLUMN config DROP DEFAULT;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE jobs ALTER COLUMN config DROP DEFAULT;
	`)
	if err != nil {
		return err
	}

	return nil
}

func CascadeTeamDeletes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE pipelines DROP CONSTRAINT pipelines_team_id_fkey;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE pipelines ADD CONSTRAINT pipelines_team_id_fkey FOREIGN KEY (team_id) REFERENCES teams (id) ON DELETE CASCADE;
	`)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`
		ALTER TABLE builds DROP CONSTRAINT builds_team_id_fkey;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE builds ADD CONSTRAINT builds_team_id_fkey FOREIGN KEY (team_id) REFERENCES teams (id) ON DELETE CASCADE;
	`)
	if err != nil {
		return err
	}

	rows, err := tx.Query(`SELECT id FROM teams`)
	if err != nil {
		return err
	}

	defer rows.Close()

	var teamIDs []int

	for rows.Next() {
		var teamID int
		err = rows.Scan(&teamID)
		if err != nil {
			return fmt.Errorf("failed to scan team ID: %s", err)
		}

		teamIDs = append(teamIDs, teamID)
	}

	for _, teamID := range teamIDs {
		err = createTeamBuildEventsTable(tx, teamID)
		if err != nil {
			return fmt.Errorf("failed to create build events table: %s", err)
		}

		err = populateTeamBuildEventsTable(tx, teamID)
		if err != nil {
			return fmt.Errorf("failed to populate build events: %s", err)
		}
	}

	// drop all constraints that depend on build_events
	_, err = tx.Exec(`
		DELETE FROM ONLY build_events
		WHERE build_id IN (SELECT id FROM builds WHERE job_id IS NULL)
	`)
	if err != nil {
		return fmt.Errorf("failed to clean up build events: %s", err)
	}

	return nil
}

func createTeamBuildEventsTable(tx migration.LimitedTx, teamID int) error {
	_, err := tx.Exec(fmt.Sprintf(`
		CREATE TABLE team_build_events_%[1]d ()
		INHERITS (build_events)
	`, teamID))
	if err != nil {
		return err
	}

	_, err = tx.Exec(fmt.Sprintf(`
		CREATE INDEX teams_build_events_%[1]d_build_id ON team_build_events_%[1]d (build_id)
	`, teamID))
	if err != nil {
		return err
	}

	_, err = tx.Exec(fmt.Sprintf(`
		CREATE UNIQUE INDEX teams_build_events_%[1]d_build_id_event_id ON team_build_events_%[1]d USING btree (build_id, event_id)
	`, teamID))
	if err != nil {
		return err
	}

	return nil
}

func populateTeamBuildEventsTable(tx migration.LimitedTx, teamID int) error {
	_, err := tx.Exec(fmt.Sprintf(`
		INSERT INTO team_build_events_%[1]d (
			build_id, type, payload, event_id, version
		)
		SELECT build_id, type, payload, event_id, version
		FROM ONLY build_events AS e, builds AS b
		WHERE b.team_id = $1
		AND b.id = e.build_id
	`, teamID), teamID)
	if err != nil {
		return fmt.Errorf("failed to insert: %s", err)
	}

	return err
}

func CreateCaches(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE pipelines
		ALTER COLUMN name SET NOT NULL
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE TYPE volume_state AS ENUM (
			'creating',
			'created',
			'destroying'
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE TYPE container_state AS ENUM (
			'creating',
			'created',
			'destroying'
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE TABLE base_resource_types (
			id serial PRIMARY KEY,
			name text NOT NULL,
			UNIQUE (name)
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE TABLE worker_base_resource_types (
			worker_name text REFERENCES workers (name) ON DELETE CASCADE,
			base_resource_type_id int REFERENCES base_resource_types (id) ON DELETE RESTRICT,
			image text NOT NULL,
			version text NOT NULL,
			UNIQUE (worker_name, base_resource_type_id)
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE TABLE resource_configs (
			id serial PRIMARY KEY,
			base_resource_type_id int REFERENCES base_resource_types (id) ON DELETE CASCADE,
			source_hash text NOT NULL
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE TABLE resource_caches (
			id serial PRIMARY KEY,
			resource_config_id int REFERENCES resource_configs (id) ON DELETE CASCADE,
			version TEXT NOT NULL,
			params_hash text NOT NULL,
			UNIQUE (resource_config_id, version, params_hash)
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE resource_configs
		ADD COLUMN resource_cache_id int REFERENCES resource_caches (id) ON DELETE CASCADE,
		ADD UNIQUE (resource_cache_id, base_resource_type_id, source_hash)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE TABLE resource_config_uses (
			resource_config_id int REFERENCES resource_configs (id) ON DELETE RESTRICT,
			build_id int REFERENCES builds (id) ON DELETE CASCADE,
			resource_id int REFERENCES resources (id) ON DELETE CASCADE,
			resource_type_id int REFERENCES resource_types (id) ON DELETE CASCADE
			-- don't bother with unique constraint; easier to just blindly insert,
			-- and allow entries to just be GCed
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE TABLE resource_cache_uses (
			resource_cache_id int REFERENCES resource_caches (id) ON DELETE RESTRICT,
			build_id int REFERENCES builds (id) ON DELETE CASCADE,
			resource_id int REFERENCES resources (id) ON DELETE CASCADE,
			resource_type_id int REFERENCES resource_types (id) ON DELETE CASCADE
			-- don't bother with unique constraint; easier to just blindly insert,
			-- and allow entries to just be GCed
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE containers
		SET build_id = NULL
		WHERE build_id NOT IN (SELECT id FROM builds)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE containers
		ALTER COLUMN handle DROP NOT NULL,
		ADD COLUMN state container_state NOT NULL DEFAULT 'created',
		ALTER COLUMN build_id SET DEFAULT NULL,
		ADD FOREIGN KEY (build_id) REFERENCES builds (id) ON DELETE SET NULL,
		ADD COLUMN resource_config_id int REFERENCES resource_configs (id) ON DELETE SET NULL,
		ADD COLUMN resource_cache_id int REFERENCES resource_caches (id) ON DELETE SET NULL,
		ADD COLUMN hijacked bool NOT NULL DEFAULT false,
		ADD CONSTRAINT handle_when_created CHECK (
			(state = 'creating' AND handle IS NULL) OR (state != 'creating')
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE containers
		ALTER COLUMN state SET DEFAULT 'creating'
	`)
	if err != nil {
		return err
	}

	// parent_state foreign key prevents any state changes on parent
	_, err = tx.Exec(`
		ALTER TABLE volumes
		ADD COLUMN resource_cache_id int REFERENCES resource_caches (id) ON DELETE SET NULL,
		ADD COLUMN base_resource_type_id int REFERENCES base_resource_types (id) ON DELETE SET NULL,
		ADD COLUMN state volume_state NOT NULL DEFAULT 'created',
		ADD COLUMN initialized bool NOT NULL DEFAULT false,
		ADD COLUMN parent_id int,
		ADD COLUMN parent_state volume_state,
		ADD UNIQUE (id, state),
		ADD FOREIGN KEY (parent_id, parent_state) REFERENCES volumes (id, state) ON DELETE RESTRICT,
		ADD CONSTRAINT cannot_invalidate_during_initialization CHECK (
			(
				state IN ('created', 'destroying') AND (
					(
						resource_cache_id IS NULL
					) AND (
						base_resource_type_id IS NULL
					) AND (
						container_id IS NULL
					)
				)
			) OR (
				(
					resource_cache_id IS NOT NULL
				) OR (
					base_resource_type_id IS NOT NULL
				) OR (
					container_id IS NOT NULL
				)
			)
		)
	`)
	if err != nil {
		return err
	}

	// https://www.pivotaltracker.com/story/show/144828721
	// All volumes that currently exist in the database have
	// already been initialized, and we rely on them being
	// initialized to GC them in the new schema.
	_, err = tx.Exec(`
		UPDATE volumes
		SET initialized = true
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE volumes
		ALTER COLUMN state SET DEFAULT 'creating'
	`)
	if err != nil {
		return err
	}

	// _, err = tx.Exec(`
	// 	WITH valid_version_ids AS (
	// 		SELECT DISTINCT version_id FROM next_build_inputs
	// 	), valid_image_resource_version_ids AS (
	// 		SELECT i.id
	// 		FROM image_resource_versions i
	// 		WHERE build_id IN (
	// 			SELECT COALESCE(MAX(id), 0) AS build_id
	// 			FROM builds
	// 			WHERE status = 'succeeded'
	// 			GROUP BY job_id
	// 		)
	// 		OR build_id IN (
	// 			SELECT COALESCE(MAX(id), 0) AS build_id
	// 			FROM builds
	// 			GROUP BY job_id
	// 		)
	// 	), newly_inserted_version_caches AS (
	// 		INSERT INTO caches (version_id)
	// 		SELECT i.version_id
	// 		FROM valid_version_ids i
	// 		LEFT JOIN caches c
	// 		ON i.version_id = c.version_id
	// 		WHERE c.image_resource_version_id IS NULL
	// 		AND c.version_id IS NULL
	// 	), newly_inserted_image_caches AS (
	// 		INSERT INTO caches (image_resource_version_id)
	// 		SELECT i.id
	// 		FROM valid_image_resource_version_ids i
	// 		LEFT JOIN caches c
	// 		ON i.id = c.image_resource_version_id
	// 		WHERE c.image_resource_version_id IS NULL
	// 		AND c.version_id IS NULL
	// 	)
	// 	DELETE FROM caches
	// 	WHERE (
	// 		version_id IS NOT NULL
	// 		AND version_id NOT IN (
	// 			SELECT version_id FROM valid_version_ids
	// 		)
	// 	) OR (
	// 		image_resource_version_id IS NOT NULL
	// 		AND image_resource_version_id NOT IN (
	// 			SELECT id FROM valid_image_resource_version_ids
	// 		)
	// 	)
	// `)
	// if err != nil {
	// 	return err
	// }

	return nil
}

func AddStateToWorkers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		CREATE TYPE worker_state AS ENUM (
			'running',
			'stalled',
			'landing',
			'landed',
			'retiring'
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE workers
		ADD COLUMN state worker_state DEFAULT 'running' NOT NULL,
		ALTER COLUMN addr DROP NOT NULL
	`)
	return err
}

func CascadeTeamDeletesOnPipes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE pipes DROP CONSTRAINT pipes_team_id_fkey;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE pipes ADD CONSTRAINT pipes_team_id_fkey FOREIGN KEY (team_id) REFERENCES teams (id) ON DELETE CASCADE;
	`)
	if err != nil {
		return err
	}

	return nil
}

func AddWorkerForeignKeyToVolumesAndContainers(tx migration.LimitedTx) error {
	var err error

	_, err = tx.Exec(`
		INSERT INTO workers (name, team_id, state, start_time, active_containers, resource_types, tags, platform, http_proxy_url, https_proxy_url, no_proxy)
			SELECT DISTINCT v.worker_name, v.team_id, 'stalled'::worker_state, 0, 0, '[]', '[]', '', '', '', ''
				FROM volumes v
				LEFT OUTER JOIN workers w
				ON w.name = v.worker_name
				WHERE w.name IS NULL
					AND v.team_id IS NULL;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO workers (name, team_id, state, start_time, active_containers, resource_types, tags, platform, http_proxy_url, https_proxy_url, no_proxy)
			SELECT DISTINCT v.worker_name, v.team_id, 'stalled'::worker_state, 0, 0, '[]', '[]', '', '', '', ''
				FROM volumes v
				LEFT OUTER JOIN workers w
				ON w.name = v.worker_name
				WHERE w.name IS NULL
					AND v.team_id IS NOT NULL;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO workers (name, team_id, state, start_time, active_containers, resource_types, tags, platform, http_proxy_url, https_proxy_url, no_proxy)
			SELECT DISTINCT v.worker_name, v.team_id, 'stalled'::worker_state, 0, 0, '[]', '[]', '', '', '', ''
				FROM containers v
				LEFT OUTER JOIN workers w
				ON w.name = v.worker_name
				WHERE w.name IS NULL
					AND v.team_id IS NULL;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO workers (name, team_id, state, start_time, active_containers, resource_types, tags, platform, http_proxy_url, https_proxy_url, no_proxy)
			SELECT DISTINCT v.worker_name, v.team_id, 'stalled'::worker_state, 0, 0, '[]', '[]', '', '', '', ''
				FROM containers v
				LEFT OUTER JOIN workers w
				ON w.name = v.worker_name
				WHERE w.name IS NULL
					AND v.team_id IS NOT NULL;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE volumes
		ADD CONSTRAINT volumes_worker_name_fkey
		FOREIGN KEY (worker_name)
		REFERENCES workers (name);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE containers
		ADD CONSTRAINT containers_worker_name_fkey
		FOREIGN KEY (worker_name)
		REFERENCES workers (name);
	`)
	if err != nil {
		return err
	}

	return nil
}

func RemoveResourceCheckingFromJobsAndAddManualyTriggeredToBuilds(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
			ALTER TABLE jobs DROP COLUMN resource_checking;
		`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
			ALTER TABLE jobs DROP COLUMN resource_check_waiver_end;
		`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
			ALTER TABLE builds ADD COLUMN manually_triggered bool DEFAULT false;
		`)
	if err != nil {
		return err
	}

	return nil

}

func RemoveTTLFromVolumes(tx migration.LimitedTx) error {
	var err error

	_, err = tx.Exec(`
		ALTER TABLE volumes
		DROP COLUMN ttl,
		DROP COLUMN expires_at;
	`)
	if err != nil {
		return err
	}

	return nil
}

func UpdateWorkerForeignKeyConstraint(tx migration.LimitedTx) error {
	var err error

	_, err = tx.Exec(`
		ALTER TABLE volumes
		DROP CONSTRAINT volumes_worker_name_fkey;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE volumes
		ADD CONSTRAINT volumes_worker_name_fkey
		FOREIGN KEY (worker_name)
		REFERENCES workers (name) ON DELETE CASCADE;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE containers
		DROP CONSTRAINT containers_worker_name_fkey;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE containers
		ADD CONSTRAINT containers_worker_name_fkey
		FOREIGN KEY (worker_name)
		REFERENCES workers (name) ON DELETE CASCADE;
	`)
	if err != nil {
		return err
	}

	return nil
}

func AddRetiringWorkerState(tx migration.LimitedTx) error {
	// Cannot delete the migration because then we would screw up the migration numbering
	return nil
}

func ReplaceBuildEventsIDWithEventID(tx migration.LimitedTx) error {
	_, err := tx.Exec(`ALTER TABLE build_events ADD COLUMN event_id integer`)
	if err != nil {
		return err
	}

	startIDs := map[int]int{}

	rows, err := tx.Query(`
		SELECT build_id, min(id)
		FROM build_events
		GROUP BY build_id
	`)
	if err != nil {
		return err
	}

	for rows.Next() {
		var buildID, id int
		err := rows.Scan(&buildID, &id)
		if err != nil {
			return err
		}

		startIDs[buildID] = id
	}

	err = rows.Close()
	if err != nil {
		return err
	}

	for buildID, id := range startIDs {
		_, err := tx.Exec(`
			UPDATE build_events
			SET event_id = id - $2
			WHERE build_id = $1
		`, buildID, id)
		if err != nil {
			return err
		}
	}

	_, err = tx.Exec(`ALTER TABLE build_events DROP COLUMN id`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`ALTER TABLE build_events ALTER COLUMN event_id SET NOT NULL`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE UNIQUE INDEX build_events_build_id_event_id ON build_events (build_id, event_id)`)
	if err != nil {
		return err
	}

	return nil
}

func AddRunningWorkerMustHaveAddrConstraint(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE workers
		ALTER COLUMN baggageclaim_url DROP NOT NULL
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE workers
    ADD CONSTRAINT addr_when_running CHECK (
			(
				state != 'stalled' AND addr IS NOT NULL AND baggageclaim_url IS NOT NULL
			) OR (
				state = 'stalled' AND addr IS NULL AND baggageclaim_url IS NULL
			)
		)
	`)
	if err != nil {
		return err
	}

	return nil
}

func AddVolumeParentIdForeignKey(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE volumes
		ADD CONSTRAINT volume_parent_id_volume_id_fkey
		FOREIGN KEY (parent_id)
		REFERENCES volumes(id)
		ON DELETE RESTRICT
    ;
	`)
	if err != nil {
		return err
	}

	return nil
}

func AddInterruptibleToJob(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE jobs
			ADD COLUMN interruptible bool NOT NULL DEFAULT false
	`)
	if err != nil {
		return err
	}

	return nil
}

func AddLandedWorkerCannotHaveAddrConstraint(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE workers
		DROP CONSTRAINT addr_when_running
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE workers SET baggageclaim_url = NULL, addr = NULL WHERE state = 'landed'
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE workers
		ADD CONSTRAINT addr_when_running CHECK (
			(
				(state != 'stalled' OR state != 'landed') AND addr IS NOT NULL AND baggageclaim_url IS NOT NULL
			) OR (
				(state = 'stalled' OR state = 'landed') AND addr IS NULL AND baggageclaim_url IS NULL
			)
		)
	`)
	if err != nil {
		return err
	}

	return nil
}

func DeleteExtraParentConstrainOnVolume(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE volumes
		DROP CONSTRAINT volume_parent_id_volume_id_fkey
    ;
	`)
	if err != nil {
		return err
	}

	return nil
}

func AddNotNullConstraintToContainerHandle(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers
		DROP CONSTRAINT handle_when_created,
		ALTER COLUMN handle SET NOT NULL
	`)
	if err != nil {
		return err
	}

	return nil
}

func FixWorkerAddrConstraint(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE workers
		DROP CONSTRAINT addr_when_running
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE workers SET baggageclaim_url = NULL, addr = NULL WHERE state = 'landed'
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE workers
		ADD CONSTRAINT addr_when_running CHECK (
			(
				(state != 'stalled' AND state != 'landed') AND (addr IS NOT NULL OR baggageclaim_url IS NOT NULL)
			) OR (
				(state = 'stalled' OR state = 'landed') AND addr IS NULL AND baggageclaim_url IS NULL
			)
		)
	`)
	if err != nil {
		return err
	}

	return nil
}

func RemoveTTLFromContainers(tx migration.LimitedTx) error {
	var err error

	_, err = tx.Exec(`
		ALTER TABLE containers
		DROP COLUMN ttl,
		DROP COLUMN expires_at;
	`)
	if err != nil {
		return err
	}

	return nil
}

func AddDiscontinuedToContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
  ALTER TABLE containers
  ADD COLUMN discontinued bool NOT NULL DEFAULT false
`)
	if err != nil {
		return err
	}
	return nil
}

func AddSourceHashToResources(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
  ALTER TABLE resources
  ADD COLUMN source_hash text
`)
	if err != nil {
		return err
	}

	rows, err := tx.Query(`SELECT id, config FROM resources`)
	if err != nil {
		return err
	}

	defer rows.Close()

	resourceSourceHashes := map[int]string{}

	for rows.Next() {
		var resourceID int
		var resourceConfigJSON string

		err = rows.Scan(&resourceID, &resourceConfigJSON)
		if err != nil {
			return fmt.Errorf("failed to scan resource ID and resource config: %s", err)
		}

		var resourceConfig atc.ResourceConfig
		err = json.Unmarshal([]byte(resourceConfigJSON), &resourceConfig)
		if err != nil {
			return fmt.Errorf("failed to unmarshal resource config: %s", err)
		}

		sourceJSON, err := json.Marshal(resourceConfig.Source)
		if err != nil {
			return fmt.Errorf("failed to marshal resource source: %s", err)
		}

		sourceHash := fmt.Sprintf("%x", sha256.Sum256(sourceJSON))
		resourceSourceHashes[resourceID] = sourceHash
	}

	for resourceID, sourceHash := range resourceSourceHashes {
		_, err = tx.Exec(`
		UPDATE resources
		SET source_hash = $1
		WHERE id = $2
	`, sourceHash, resourceID)
		if err != nil {
			return err
		}
	}

	_, err = tx.Exec(`
	ALTER TABLE resources
	ALTER COLUMN source_hash SET NOT NULL
`)
	if err != nil {
		return err
	}

	return nil
}

func AddWorkerBaseResourceTypeIdToContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
	  ALTER TABLE worker_base_resource_types
		ADD COLUMN id SERIAL PRIMARY KEY;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE containers
		ADD COLUMN worker_base_resource_types_id INTEGER
		REFERENCES worker_base_resource_types (id) ON DELETE SET NULL;
	`)
	if err != nil {
		return err
	}

	return nil
}

func AddInterceptibleToBuilds(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
	  ALTER TABLE builds
		ADD COLUMN interceptible BOOLEAN DEFAULT TRUE;
	`)
	if err != nil {
		return err
	}

	return nil
}

func AlterExpiresToIncludeTimezoneInWorkers(tx migration.LimitedTx) error {

	_, err := tx.Exec(`
		ALTER TABLE workers
		ALTER COLUMN expires type timestamp with time zone;
	`)
	if err != nil {
		return err
	}

	return nil
}

func ChangeVolumeBaseResourceTypeToWorkerBaseResourceType(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
      ALTER TABLE volumes
      ADD COLUMN worker_base_resource_type_id int REFERENCES worker_base_resource_types (id) ON DELETE SET NULL
		`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
      UPDATE volumes v
      SET worker_base_resource_type_id=(SELECT id FROM worker_base_resource_types w WHERE v.worker_name = w.worker_name AND v.base_resource_type_id = w.base_resource_type_id)
      WHERE v.base_resource_type_id IS NOT NULL
    `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE volumes
    DROP COLUMN base_resource_type_id
  `)
	if err != nil {
		return err
	}

	return nil
}

func AddLocks(tx migration.LimitedTx) error {
	_, err := tx.Exec(`CREATE TABLE locks (
      id serial PRIMARY KEY,
      name text NOT NULL,
			UNIQUE (name)
	)`)
	if err != nil {
		return err
	}

	return nil
}

func RenameWorkerBaseResourceTypesId(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
      ALTER TABLE containers
      RENAME COLUMN worker_base_resource_types_id TO worker_base_resource_type_id
		`)
	if err != nil {
		return err
	}

	return nil
}

func AddWorkerResourceCacheToContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    CREATE TABLE worker_resource_caches (
      id serial PRIMARY KEY,
      worker_base_resource_type_id int REFERENCES worker_base_resource_types (id) ON DELETE CASCADE,
      resource_cache_id int REFERENCES resource_caches (id) ON DELETE CASCADE
    )
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
      ALTER TABLE containers
      ADD COLUMN worker_resource_cache_id INTEGER
  		REFERENCES worker_resource_caches (id) ON DELETE SET NULL
		`)
	if err != nil {
		return err
	}

	rows, err := tx.Query(`SELECT id, resource_cache_id, worker_name FROM containers WHERE resource_cache_id IS NOT NULL`)
	if err != nil {
		return err
	}

	defer rows.Close()

	containerWorkerResourceCaches := []containerWorkerResourceCache{}

	for rows.Next() {
		var id int
		var resourceCacheID int
		var workerName string
		err = rows.Scan(&id, &resourceCacheID, &workerName)
		if err != nil {
			return fmt.Errorf("failed to scan container id, resource_cache_id and worker_name: %s", err)
		}

		containerWorkerResourceCaches = append(containerWorkerResourceCaches, containerWorkerResourceCache{
			ID:              id,
			ResourceCacheID: resourceCacheID,
			WorkerName:      workerName,
		})
	}

	for _, cwrc := range containerWorkerResourceCaches {
		baseResourceTypeID, err := findBaseResourceTypeID(tx, cwrc.ResourceCacheID)
		if err != nil {
			return err
		}
		if baseResourceTypeID == 0 {
			// most likely resource cache was garbage collected
			// keep worker_base_resource_type_id as null, so that gc can remove this container
			continue
		}

		var workerBaseResourceTypeID int
		err = tx.QueryRow(`
      SELECT id FROM worker_base_resource_types WHERE base_resource_type_id=$1 AND worker_name=$2
    `, baseResourceTypeID, cwrc.WorkerName).
			Scan(&workerBaseResourceTypeID)
		if err != nil {
			return err
		}

		var workerResourceCacheID int
		err = tx.QueryRow(`
				SELECT id FROM worker_resource_caches WHERE worker_base_resource_type_id = $1 AND resource_cache_id = $2
			`, workerBaseResourceTypeID, cwrc.ResourceCacheID).
			Scan(&workerResourceCacheID)
		if err != nil {
			if err != sql.ErrNoRows {
				return err
			}

			err = tx.QueryRow(`
				INSERT INTO worker_resource_caches (worker_base_resource_type_id, resource_cache_id)
		    VALUES ($1, $2)
		    RETURNING id
			`, workerBaseResourceTypeID, cwrc.ResourceCacheID).
				Scan(&workerResourceCacheID)
			if err != nil {
				return err
			}
		}

		_, err = tx.Exec(`
        UPDATE containers SET worker_resource_cache_id=$1 WHERE id=$2
      `, workerResourceCacheID, cwrc.ID)
		if err != nil {
			return err
		}
	}

	_, err = tx.Exec(`
      ALTER TABLE containers
      DROP COLUMN resource_cache_id
    `)
	if err != nil {
		return err
	}

	return nil
}

type containerWorkerResourceCache struct {
	ID              int
	ResourceCacheID int
	WorkerName      string
}

func findBaseResourceTypeID(tx migration.LimitedTx, resourceCacheID int) (int, error) {
	var innerResourceCacheID sql.NullInt64
	var baseResourceTypeID sql.NullInt64

	err := tx.QueryRow(`
    SELECT resource_cache_id, base_resource_type_id FROM resource_caches rca LEFT JOIN resource_configs rcf ON rca.resource_config_id = rcf.id WHERE rca.id=$1
  `, resourceCacheID).
		Scan(&innerResourceCacheID, &baseResourceTypeID)
	if err != nil {
		return 0, err
	}

	if baseResourceTypeID.Valid {
		return int(baseResourceTypeID.Int64), nil
	}

	if innerResourceCacheID.Valid {
		return findBaseResourceTypeID(tx, int(innerResourceCacheID.Int64))
	}

	return 0, nil
}

func AddWorkerResourceCacheToVolumes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
      ALTER TABLE volumes
      ADD COLUMN worker_resource_cache_id INTEGER
  		REFERENCES worker_resource_caches (id) ON DELETE SET NULL
		`)
	if err != nil {
		return err
	}

	rows, err := tx.Query(`SELECT id, resource_cache_id, worker_name FROM volumes WHERE resource_cache_id IS NOT NULL`)
	if err != nil {
		return err
	}

	defer rows.Close()

	volumeWorkerResourceCaches := []volumeWorkerResourceCache{}

	for rows.Next() {
		var id int
		var resourceCacheID int
		var workerName string
		err = rows.Scan(&id, &resourceCacheID, &workerName)
		if err != nil {
			return fmt.Errorf("failed to scan volume id, resource_cache_id and worker_name: %s", err)
		}

		volumeWorkerResourceCaches = append(volumeWorkerResourceCaches, volumeWorkerResourceCache{
			ID:              id,
			ResourceCacheID: resourceCacheID,
			WorkerName:      workerName,
		})
	}

	for _, vwrc := range volumeWorkerResourceCaches {
		baseResourceTypeID, err := findBaseResourceTypeID(tx, vwrc.ResourceCacheID)
		if err != nil {
			return err
		}
		if baseResourceTypeID == 0 {
			// most likely resource cache was garbage collected
			// keep worker_base_resource_type_id as null, so that gc can remove this container
			continue
		}

		var workerBaseResourceTypeID int
		err = tx.QueryRow(`
		      SELECT id FROM worker_base_resource_types WHERE base_resource_type_id=$1 AND worker_name=$2
		    `, baseResourceTypeID, vwrc.WorkerName).
			Scan(&workerBaseResourceTypeID)
		if err != nil {
			return err
		}

		var workerResourceCacheID int
		err = tx.QueryRow(`
				SELECT id FROM worker_resource_caches WHERE worker_base_resource_type_id = $1 AND resource_cache_id = $2
			`, workerBaseResourceTypeID, vwrc.ResourceCacheID).
			Scan(&workerResourceCacheID)
		if err != nil {
			if err != sql.ErrNoRows {
				return err
			}

			err = tx.QueryRow(`
				INSERT INTO worker_resource_caches (worker_base_resource_type_id, resource_cache_id)
		    VALUES ($1, $2)
		    RETURNING id
			`, workerBaseResourceTypeID, vwrc.ResourceCacheID).
				Scan(&workerResourceCacheID)
			if err != nil {
				return err
			}
		}

		_, err = tx.Exec(`
        UPDATE volumes SET worker_resource_cache_id=$1 WHERE id=$2
      `, workerResourceCacheID, vwrc.ID)
		if err != nil {
			return err
		}
	}

	_, err = tx.Exec(`
    ALTER TABLE volumes
    DROP COLUMN resource_cache_id,
		ADD CONSTRAINT cannot_invalidate_during_initialization CHECK (
			(
				state IN ('created', 'destroying') AND (
					(
						worker_resource_cache_id IS NULL
					) AND (
						worker_base_resource_type_id IS NULL
					) AND (
						container_id IS NULL
					)
				)
			) OR (
				(
					worker_resource_cache_id IS NOT NULL
				) OR (
					worker_base_resource_type_id IS NOT NULL
				) OR (
					container_id IS NOT NULL
				)
			)
		)
  `)
	if err != nil {
		return err
	}

	return nil
}

type volumeWorkerResourceCache struct {
	ID              int
	ResourceCacheID int
	WorkerName      string
}

func RemoveLastTrackedFromBuilds(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE builds
    DROP COLUMN last_tracked
  `)
	if err != nil {
		return err
	}

	return nil
}

// We pretty much just added an index for every foreign key, plus one on
// containers for plan_id.
//
// We'll check later which ones are wasteful.
//
// We promise.
//
// Useful queries:
//
// Show fkeys without indexes:
//
//     CREATE FUNCTION pg_temp.sortarray(int2[]) returns int2[] as '
//       SELECT ARRAY(
//           SELECT $1[i]
//             FROM generate_series(array_lower($1, 1), array_upper($1, 1)) i
//         ORDER BY 1
//       )
//     ' language sql;

//     SELECT conrelid::regclass, conname, reltuples::bigint
//     FROM pg_constraint
//     JOIN pg_class ON (conrelid = pg_class.oid)
//     WHERE contype = 'f'
//     AND NOT EXISTS (
//       SELECT 1
//       FROM pg_index
//       WHERE indrelid = conrelid
//       AND pg_temp.sortarray(conkey) = pg_temp.sortarray(indkey)
//     )
//     ORDER BY reltuples DESC;
//
// Show size and # of hits for each index:
//
//     SELECT
//       t.tablename,
//       indexname,
//       c.reltuples AS num_rows,
//       pg_size_pretty(pg_relation_size(quote_ident(t.tablename)::text)) AS table_size,
//       pg_size_pretty(pg_relation_size(quote_ident(indexrelname)::text)) AS index_size,
//       CASE WHEN indisunique THEN 'Y'
//         ELSE 'N'
//       END AS UNIQUE,
//       idx_scan AS number_of_scans,
//       idx_tup_read AS tuples_read,
//       idx_tup_fetch AS tuples_fetched
//     FROM pg_tables t
//     LEFT OUTER JOIN pg_class c ON t.tablename=c.relname
//     LEFT OUTER JOIN (
//       SELECT
//         c.relname AS ctablename,
//         ipg.relname AS indexname,
//         x.indnatts AS number_of_columns,
//         idx_scan,
//         idx_tup_read,
//         idx_tup_fetch,
//         indexrelname,
//         indisunique
//       FROM pg_index x
//       JOIN pg_class c ON c.oid = x.indrelid
//       JOIN pg_class ipg ON ipg.oid = x.indexrelid
//       JOIN pg_stat_all_indexes psai ON x.indexrelid = psai.indexrelid
//     ) AS foo ON t.tablename = foo.ctablename
//     WHERE t.schemaname='public'
//     ORDER BY 1, 2;

func AddIndexesToABunchMoreStuff(tx migration.LimitedTx) error {
	_, err := tx.Exec(`CREATE INDEX builds_team_id ON builds (team_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX independent_build_inputs_job_id ON independent_build_inputs (job_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX independent_build_inputs_version_id ON independent_build_inputs (version_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX next_build_inputs_job_id ON next_build_inputs (job_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX next_build_inputs_version_id ON next_build_inputs (version_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX resource_cache_uses_resource_cache_id ON resource_cache_uses (resource_cache_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX resource_cache_uses_build_id ON resource_cache_uses (build_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX resource_cache_uses_resource_type_id ON resource_cache_uses (resource_type_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX resource_cache_uses_resource_id ON resource_cache_uses (resource_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX resource_caches_resource_config_id ON resource_caches (resource_config_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX resource_config_uses_resource_config_id ON resource_config_uses (resource_config_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX resource_config_uses_build_id ON resource_config_uses (build_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX resource_config_uses_resource_type_id ON resource_config_uses (resource_type_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX resource_config_uses_resource_id ON resource_config_uses (resource_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX resource_configs_base_resource_type_id ON resource_configs (base_resource_type_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX resource_configs_resource_cache_id ON resource_configs (resource_cache_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX containers_resource_config_id ON containers (resource_config_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX containers_worker_resource_cache_id ON containers (worker_resource_cache_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX containers_build_id ON containers (build_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX containers_plan_id ON containers (plan_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX containers_team_id ON containers (team_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX containers_worker_name ON containers (worker_name)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX volumes_container_id ON volumes (container_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX volumes_parent_id ON volumes (parent_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX volumes_team_id ON volumes (team_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX volumes_worker_resource_cache_id ON volumes (worker_resource_cache_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX volumes_worker_base_resource_type_id ON volumes (worker_base_resource_type_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX worker_resource_caches_worker_base_resource_type_id ON worker_resource_caches (worker_base_resource_type_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX worker_resource_caches_resource_cache_id ON worker_resource_caches (resource_cache_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX worker_base_resource_types_base_resource_type_id ON worker_base_resource_types (base_resource_type_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX worker_base_resource_types_worker_name ON worker_base_resource_types (worker_name)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX workers_team_id ON workers (team_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX resource_types_pipeline_id ON resource_types (pipeline_id)`)
	if err != nil {
		return err
	}

	return nil
}

func RemoveDuplicateIndices(tx migration.LimitedTx) error {
	_, err := tx.Exec(`DROP INDEX builds_job_id_idx, jobs_pipeline_id_idx, resources_pipeline_id_idx, pipelines_team_id_idx`)
	if err != nil {
		return err
	}

	return nil
}

func CleanUpContainerColumns(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers
		ADD COLUMN meta_type text NOT NULL DEFAULT '',
		ADD COLUMN meta_step_name text NOT NULL DEFAULT '',
		ADD COLUMN meta_attempt text NOT NULL DEFAULT '',
		ADD COLUMN meta_working_directory text NOT NULL DEFAULT '',
		ADD COLUMN meta_process_user text NOT NULL DEFAULT '',
		ADD COLUMN meta_pipeline_id integer NOT NULL DEFAULT 0,
		ADD COLUMN meta_job_id integer NOT NULL DEFAULT 0,
		ADD COLUMN meta_build_id integer NOT NULL DEFAULT 0,
		ADD COLUMN meta_pipeline_name text NOT NULL DEFAULT '',
		ADD COLUMN meta_job_name text NOT NULL DEFAULT '',
		ADD COLUMN meta_build_name text NOT NULL DEFAULT '',
		DROP COLUMN type,
		DROP COLUMN step_name,
		DROP COLUMN check_type,
		DROP COLUMN check_source,
		DROP COLUMN working_directory,
		DROP COLUMN env_variables,
		DROP COLUMN attempts,
		DROP COLUMN stage,
		DROP COLUMN image_resource_type,
		DROP COLUMN image_resource_source,
		DROP COLUMN process_user,
		DROP COLUMN resource_type_version
	`)
	if err != nil {
		return err
	}

	return nil
}

func AddAuthToTeams(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE teams
    ADD COLUMN auth json NULL;
	`)
	if err != nil {
		return err
	}

	rows, err := tx.Query(`
		SELECT id, github_auth, uaa_auth, genericoauth_auth
		FROM teams
	`)

	if err != nil {
		return err
	}

	defer rows.Close()

	teamConfigs := map[int][]byte{}

	for rows.Next() {
		var (
			id int

			githubAuth, uaaAuth, genericOAuth sql.NullString
		)

		err := rows.Scan(&id, &githubAuth, &uaaAuth, &genericOAuth)
		if err != nil {
			return err
		}

		authConfigs := make(map[string]*json.RawMessage)

		if githubAuth.Valid && githubAuth.String != "null" {
			data := []byte(githubAuth.String)
			authConfigs["github"] = (*json.RawMessage)(&data)
		}

		if uaaAuth.Valid && uaaAuth.String != "null" {
			data := []byte(uaaAuth.String)
			authConfigs["uaa"] = (*json.RawMessage)(&data)
		}

		if genericOAuth.Valid && genericOAuth.String != "null" {
			data := []byte(genericOAuth.String)
			authConfigs["oauth"] = (*json.RawMessage)(&data)
		}

		jsonConfig, err := json.Marshal(authConfigs)
		if err != nil {
			return err
		}

		teamConfigs[id] = jsonConfig
	}

	for id, jsonConfig := range teamConfigs {
		_, err = tx.Exec(`
			UPDATE teams
			SET auth = $1
			WHERE id = $2
		`, jsonConfig, id)
		if err != nil {
			return err
		}
	}

	_, err = tx.Exec(`
		ALTER TABLE teams DROP COLUMN github_auth, DROP COLUMN uaa_auth, DROP COLUMN genericoauth_auth;
	`)
	if err != nil {
		return err
	}

	return nil
}

func AddCertificatesPathToWorkers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE workers
		ADD COLUMN certificates_path text,
		ADD COLUMN certificates_symlinked_paths json NOT NULL DEFAULT 'null';
`)
	return err
}

func RemoveCertificatesPathToWorkers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE workers
		DROP COLUMN certificates_path,
		DROP COLUMN certificates_symlinked_paths;
`)
	return err
}

func AddVersionToWorkers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE workers
		ADD COLUMN version text;
`)
	return err
}

func AddConfig(tx migration.LimitedTx) error {
	_, err := tx.Exec(`CREATE TABLE config (config text NOT NULL)`)
	if err != nil {
		return err
	}

	return nil
}

func AddNonceToTeams(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE teams
		ADD COLUMN nonce text;
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE teams
		ALTER COLUMN auth TYPE text;
`)
	if err != nil {
		return err
	}

	return nil
}

func AddNonceToResourcesAndResourceTypes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE resources
		ADD COLUMN nonce text;
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE resources
		ALTER COLUMN config TYPE text;
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE resource_types
		ADD COLUMN nonce text;
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE resource_types
		ALTER COLUMN config TYPE text;
`)
	if err != nil {
		return err
	}

	return nil
}

func AddMetadataToResourceCache(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE resource_caches
		ADD COLUMN metadata text
	`)
	if err != nil {
		return err
	}

	resourceCacheRows, err := tx.Query(`SELECT
		rca.id, version, source_hash, rcfg.base_resource_type_id, rcfg.resource_cache_id
		FROM resource_caches rca
		LEFT JOIN resource_configs rcfg
		ON rca.resource_config_id=rcfg.id`,
	)
	if err != nil {
		return err
	}

	defer resourceCacheRows.Close()

	resourceCacheInfos := []resourceCacheInfo{}

	for resourceCacheRows.Next() {
		var info resourceCacheInfo

		err = resourceCacheRows.Scan(&info.id, &info.version, &info.sourceHash, &info.baseResourceTypeID, &info.resourceCacheID)
		if err != nil {
			return err
		}
		resourceCacheInfos = append(resourceCacheInfos, info)
	}

	resourceTypesRows, err := tx.Query(`SELECT
		name, version, config
		FROM resource_types`,
	)
	if err != nil {
		return err
	}

	defer resourceTypesRows.Close()

	resourceTypesMap := make(resourceTypesInfo)

	for resourceTypesRows.Next() {
		var name string
		var version sql.NullString
		var config string

		err = resourceTypesRows.Scan(&name, &version, &config)
		if err != nil {
			return err
		}
		var resourceConfig atc.ResourceConfig
		err = json.Unmarshal([]byte(config), &resourceConfig)
		if err != nil {
			return err
		}

		sourceJSON, err := json.Marshal(resourceConfig.Source)
		if err != nil {
			return err
		}
		sourceHash := fmt.Sprintf("%x", sha256.Sum256(sourceJSON))

		if !version.Valid {
			// version is not set on resource type, ignore it, it does not have cached resources yet
			continue
		}

		resourceTypesMap[resourceTypeInfoKey{
			sourceHash: sourceHash,
			version:    version.String,
		}] = name
	}

	for _, info := range resourceCacheInfos {
		var resourceTypeName string
		// Base Resource Type
		if info.baseResourceTypeID.Valid {
			err = tx.QueryRow(
				"SELECT name FROM base_resource_types WHERE id = $1",
				info.baseResourceTypeID.Int64,
			).Scan(&resourceTypeName)
			if err != nil {
				return err
			}
		} else {
			var parentInfo resourceCacheInfo

			err = tx.QueryRow(
				`SELECT
				rca.id, version, source_hash, rcfg.base_resource_type_id, rcfg.resource_cache_id
				FROM resource_caches rca
				LEFT JOIN resource_configs rcfg
				ON rca.resource_config_id=rcfg.id
				WHERE rca.id = $1`,
				info.resourceCacheID.Int64,
			).Scan(
				&parentInfo.id, &parentInfo.version, &parentInfo.sourceHash, &parentInfo.baseResourceTypeID, &parentInfo.resourceCacheID,
			)
			if err != nil {
				return err
			}

			var ok bool
			resourceTypeName, ok = resourceTypesMap[resourceTypeInfoKey{
				sourceHash: parentInfo.sourceHash,
				version:    parentInfo.version,
			}]
			if !ok {
				// the resource type was deleted, while resource cache still exists
				continue
			}
		}

		var metadata string
		err = tx.QueryRow(
			"SELECT metadata FROM versioned_resources vr LEFT JOIN resources r ON vr.resource_id = r.id WHERE source_hash = $1 AND version = $2 AND type = $3",
			info.sourceHash,
			info.version,
			resourceTypeName,
		).Scan(&metadata)
		if err != nil {
			if err == sql.ErrNoRows {
				// this is custom resource type, which has resource cache but does not have versioned_resource
				continue
			}
			return err
		}

		_, err = tx.Exec("UPDATE resource_caches SET metadata = $1 WHERE id = $2", metadata, info.id)
		if err != nil {
			return err
		}
	}

	return nil
}

type resourceTypesInfo map[resourceTypeInfoKey]string

type resourceTypeInfoKey struct {
	sourceHash string
	version    string
}

type resourceCacheInfo struct {
	id                 int
	version            string
	sourceHash         string
	baseResourceTypeID sql.NullInt64
	resourceCacheID    sql.NullInt64
}

func AddNonceToJobs(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE jobs
		ADD COLUMN nonce text;
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE jobs
		ALTER COLUMN config TYPE text;
`)
	if err != nil {
		return err
	}

	return nil
}

func AddNonceToPipelines(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE pipelines
		ADD COLUMN nonce text;
`)
	if err != nil {
		return err
	}

	return nil
}

func AddCreatingContainerIDAndStateToContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers
		ADD COLUMN creating_container_id integer REFERENCES containers (id) ON DELETE SET NULL
`)
	if err != nil {
		return err
	}

	return nil
}

func ReplaceCreatingContainerIDWithImageCheckForContainerIDAndImageGetForContainerID(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers
		ADD COLUMN image_check_container_id integer REFERENCES containers (id) ON DELETE SET NULL,
		ADD COLUMN image_get_container_id integer REFERENCES containers (id) ON DELETE SET NULL,
		DROP COLUMN creating_container_id
`)
	if err != nil {
		return err
	}

	return nil
}

func DropWorkerResourceCacheFromContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers
		DROP COLUMN worker_resource_cache_id
`)
	if err != nil {
		return err
	}

	return nil
}

func AddResourceConfigCheckSessions(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		CREATE TABLE resource_config_check_sessions (
			id serial PRIMARY KEY,
			resource_config_id integer REFERENCES resource_configs (id) ON DELETE CASCADE,
			worker_base_resource_type_id integer REFERENCES worker_base_resource_types (id) ON DELETE CASCADE,
			expires_at timestamp with time zone
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE containers
		ADD COLUMN resource_config_check_session_id integer REFERENCES resource_config_check_sessions (id) ON DELETE SET NULL,
		DROP COLUMN resource_config_id,
		DROP COLUMN worker_base_resource_type_id
	`)
	if err != nil {
		return err
	}

	return nil
}

func CreateContainerGCIndexes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`CREATE INDEX containers_image_check_container_id ON containers (image_check_container_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX containers_image_get_container_id ON containers (image_get_container_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX containers_resource_config_check_session_id ON containers (resource_config_check_session_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX resource_config_check_sessions_resource_config_id ON resource_config_check_sessions (resource_config_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX resource_config_check_sessions_worker_base_resource_type_id ON resource_config_check_sessions (worker_base_resource_type_id)`)
	if err != nil {
		return err
	}

	return nil
}

func AddIndexesForBuildCollector(tx migration.LimitedTx) error {
	_, err := tx.Exec(`CREATE INDEX builds_completed ON builds (completed)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX builds_status ON builds (status)`)
	if err != nil {
		return err
	}

	return nil
}

func DropOldLocks(tx migration.LimitedTx) error {
	_, err := tx.Exec(`DROP TABLE resource_checking_lock`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`DROP TABLE build_scheduling_lock`)
	if err != nil {
		return err
	}

	return nil
}

func DropInitializedFromVolumes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE volumes
		DROP COLUMN initialized
`)
	if err != nil {
		return err
	}

	return nil
}

func AddUniqueWorkerResourceCacheIDToVolumes(tx migration.LimitedTx) error {
	// handle initialized volumes first to avoid
	// 'cannot_invalidate_during_initialization'
	_, err := tx.Exec(`
		WITH distinct_vols AS (
			SELECT DISTINCT ON (worker_resource_cache_id) id
			FROM volumes
			WHERE worker_resource_cache_id IS NOT NULL
		)
		UPDATE volumes
		SET worker_resource_cache_id = NULL, state = 'destroying'
		WHERE worker_resource_cache_id IS NOT NULL
		AND state = 'creating'
		AND id NOT IN (SELECT id FROM distinct_vols)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		WITH distinct_vols AS (
			SELECT DISTINCT ON (worker_resource_cache_id) id
			FROM volumes
			WHERE worker_resource_cache_id IS NOT NULL
		)
		UPDATE volumes
		SET worker_resource_cache_id = NULL
		WHERE worker_resource_cache_id IS NOT NULL
		AND id NOT IN (SELECT id FROM distinct_vols)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE UNIQUE INDEX volumes_worker_resource_cache_unique
		ON volumes (worker_resource_cache_id)
	`)
	if err != nil {
		return err
	}

	return nil
}

func AddBuildImageResourceCaches(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		CREATE TABLE build_image_resource_caches (
			resource_cache_id integer REFERENCES resource_caches (id) ON DELETE RESTRICT,
			build_id integer NOT NULL REFERENCES builds (id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		DROP TABLE image_resource_versions
	`)
	if err != nil {
		return err
	}

	return nil
}

func AddNonceAndPublicPlanToBuilds(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE builds
		ADD COLUMN nonce text;
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE builds
		ADD COLUMN public_plan json DEFAULT '{}';
`)
	if err != nil {
		return err
	}

	offset := 0
	for {
		rows, err := tx.Query(fmt.Sprintf(`
		SELECT id, engine_metadata
		FROM builds
		WHERE engine='exec.v2'
		ORDER BY id ASC
		LIMIT 500
		OFFSET %d
	`, offset))
		if err != nil {
			return err
		}

		defer rows.Close()

		//create public plans
		plans := map[int]internal_163.Plan{}

		totalRows := 0
		for rows.Next() {
			totalRows++

			var buildID int
			var engineMetadataJSON []byte
			err := rows.Scan(&buildID, &engineMetadataJSON)
			if err != nil {
				return err
			}

			if engineMetadataJSON == nil {
				continue
			}

			var execEngineMetadata execV2Metadata
			err = json.Unmarshal(engineMetadataJSON, &execEngineMetadata)
			if err != nil {
				return err
			}

			plans[buildID] = execEngineMetadata.Plan
		}

		if totalRows == 0 {
			break
		} else {
			offset += totalRows
		}

		for buildID, plan := range plans {
			_, err := tx.Exec(`
				UPDATE builds
				SET
				  public_plan = $1
				WHERE
					id = $2
			`, plan.Public(), buildID)
			if err != nil {
				return err
			}
		}
	}

	_, err = tx.Exec(`
		UPDATE builds
		  SET
			  engine_metadata = NULL
			WHERE
			  engine = 'exec.v2' AND
				status IN ('succeeded','aborted','failed','errored')
	`)

	return err
}

type execV2Metadata struct {
	Plan internal_163.Plan
}

func AddWorkerTaskCaches(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    CREATE TABLE worker_task_caches (
      id serial PRIMARY KEY,
      worker_name text REFERENCES workers (name) ON DELETE CASCADE,
      job_id int REFERENCES jobs (id) ON DELETE CASCADE,
      step_name text NOT NULL,
      path text NOT NULL
    )
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
      ALTER TABLE volumes
      ADD COLUMN worker_task_cache_id int REFERENCES worker_task_caches (id) ON DELETE SET NULL,
      DROP CONSTRAINT cannot_invalidate_during_initialization
		`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE volumes
    ADD CONSTRAINT cannot_invalidate_during_initialization CHECK (
    (
      state IN ('created', 'destroying') AND (
      (
        worker_resource_cache_id IS NULL
      ) AND (
        worker_base_resource_type_id IS NULL
      ) AND (
        worker_task_cache_id IS NULL
      ) AND (
        container_id IS NULL
      )
    )
      ) OR (
        (
          worker_resource_cache_id IS NOT NULL
        ) OR (
          worker_base_resource_type_id IS NOT NULL
        ) OR (
          worker_task_cache_id IS NOT NULL
        ) OR (
          container_id IS NOT NULL
        )
      )
    )
  `)
	if err != nil {
		return err
	}

	return nil
}

func AddGroupsAndRemoveConfigFromPipeline(strategy EncryptionStrategy) migration.Migrator {
	return func(tx migration.LimitedTx) error {
		_, err := tx.Exec(`
			ALTER TABLE pipelines
			ADD COLUMN groups json;
		`)
		if err != nil {
			return err
		}

		rows, err := tx.Query(`
			SELECT id, config, nonce
			FROM pipelines
		`)
		if err != nil {
			return err
		}

		defer rows.Close()

		pipelineGroups := map[int][]byte{}
		for rows.Next() {
			var (
				pipelineID     int
				pipelineConfig []byte
				nonce          sql.NullString
			)

			err := rows.Scan(&pipelineID, &pipelineConfig, &nonce)
			if err != nil {
				return err
			}

			var noncense *string
			if nonce.Valid {
				noncense = &nonce.String
			}

			decryptedConfig, err := strategy.Decrypt(string(pipelineConfig), noncense)
			if err != nil {
				return err
			}

			var config atc.Config
			err = json.Unmarshal(decryptedConfig, &config)
			if err != nil {
				return err
			}

			groups, err := json.Marshal(config.Groups)
			if err != nil {
				return err
			}

			pipelineGroups[pipelineID] = groups
		}

		for id, groups := range pipelineGroups {
			_, err := tx.Exec(`
		UPDATE pipelines
		SET groups = $1
		WHERE id = $2`, groups, id)
			if err != nil {
				return err
			}
		}

		_, err = tx.Exec(`
		ALTER TABLE pipelines
		DROP COLUMN config
	`)
		if err != nil {
			return err
		}

		_, err = tx.Exec(`
		ALTER TABLE pipelines
		DROP COLUMN nonce
	`)
		if err != nil {
			return err
		}

		return nil
	}
}

func AddPipelineIdToBuilds(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE builds
  	ADD COLUMN pipeline_id integer,
  	ADD FOREIGN KEY (pipeline_id) REFERENCES pipelines(id) ON DELETE CASCADE;
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
  	UPDATE builds
    SET pipeline_id = (SELECT j.pipeline_id FROM jobs j WHERE j.id = builds.job_id);
		`)
	if err != nil {
		return err
	}

	return nil
}

func UseMd5ForVersions(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		DROP INDEX versioned_resources_resource_id_type_version;
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE UNIQUE INDEX versioned_resources_resource_id_type_version
		ON versioned_resources (resource_id, type, md5(version));
`)
	if err != nil {
		return err
	}

	return nil
}

func RemoveSizeInBytesFromVolumes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE volumes DROP size_in_bytes;
	`)

	if err != nil {
		return err
	}

	return nil
}

func AddOnDeleteRestrictToResourceConfigsAndCachesAndResourceConfigCheckSessions(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE resource_configs
    DROP CONSTRAINT resource_configs_resource_cache_id_fkey;
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE resource_configs
    ADD CONSTRAINT resource_configs_resource_cache_id_fkey
    FOREIGN KEY (resource_cache_id)
    REFERENCES resource_caches (id) ON DELETE RESTRICT;
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE resource_caches
    DROP CONSTRAINT resource_caches_resource_config_id_fkey;
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE resource_caches
    ADD CONSTRAINT resource_caches_resource_config_id_fkey
    FOREIGN KEY (resource_config_id)
    REFERENCES resource_configs (id) ON DELETE RESTRICT;
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE resource_config_check_sessions
    DROP CONSTRAINT resource_config_check_sessions_resource_config_id_fkey;
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE resource_config_check_sessions
    ADD CONSTRAINT resource_config_check_sessions_resource_config_id_fkey
    FOREIGN KEY (resource_config_id)
    REFERENCES resource_configs (id) ON DELETE RESTRICT;
  `)
	if err != nil {
		return err
	}

	return nil
}

func AddNameToBuildInputs(tx migration.LimitedTx) error {
	_, err := tx.Exec(`ALTER TABLE build_inputs ADD COLUMN name text`)
	if err != nil {
		return err
	}

	names := map[int]string{}

	rows, err := tx.Query(`
    SELECT i.versioned_resource_id, v.resource_name
    FROM build_inputs i, versioned_resources v
    WHERE v.id = i.versioned_resource_id
  `)
	if err != nil {
		return err
	}

	defer rows.Close()

	for rows.Next() {
		var vrID int
		var name string
		err := rows.Scan(&vrID, &name)
		if err != nil {
			return err
		}

		names[vrID] = name
	}

	for vrID, name := range names {
		_, err := tx.Exec(`
      UPDATE build_inputs
      SET name = $2
      WHERE versioned_resource_id = $1
    `, vrID, name)
		if err != nil {
			return err
		}
	}

	_, err = tx.Exec(`ALTER TABLE build_inputs ALTER COLUMN name SET NOT NULL`)
	if err != nil {
		return err
	}

	return nil
}

func AddWorkerResourceConfigCheckSessions(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    CREATE TABLE worker_resource_config_check_sessions (
      id serial PRIMARY KEY,
      worker_base_resource_type_id integer REFERENCES worker_base_resource_types (id) ON DELETE CASCADE,
			resource_config_check_session_id integer REFERENCES resource_config_check_sessions (id) ON DELETE CASCADE
    )
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE resource_config_check_sessions
    DROP COLUMN worker_base_resource_type_id
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE containers
    DROP COLUMN resource_config_check_session_id
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE containers
    ADD COLUMN worker_resource_config_check_session_id integer REFERENCES worker_resource_config_check_sessions (id) ON DELETE SET NULL
  `)
	if err != nil {
		return err
	}

	return nil
}

func DropResourceConfigUsesAndAddContainerIDToResourceCacheUsesWhileAlsoRemovingResourceIDAndResourceTypeIDFromResourceCacheUses(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    DROP TABLE resource_config_uses
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE resource_cache_uses
    DROP COLUMN resource_id,
    DROP COLUMN resource_type_id,
		ADD COLUMN container_id integer REFERENCES containers (id) ON DELETE CASCADE
  `)
	if err != nil {
		return err
	}

	return nil
}

func AddResourceConfigIDToResources(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE resources
		DROP COLUMN source_hash,
		ADD COLUMN resource_config_id integer REFERENCES resource_configs (id) ON DELETE SET NULL
  `)
	if err != nil {
		return err
	}

	return nil
}

func AddResourceConfigIDToResourceTypes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE resource_types
		ADD COLUMN resource_config_id integer REFERENCES resource_configs (id) ON DELETE SET NULL
  `)
	if err != nil {
		return err
	}

	return nil
}

func AddIndexesToEvenMoreForeignKeys(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		CREATE INDEX builds_pipeline_id ON builds (pipeline_id)
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX resource_types_resource_config_id ON resource_types (resource_config_id)
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX resource_cache_uses_container_id ON resource_cache_uses (container_id)
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX worker_resource_config_check_sessions_resource_config_check_session_id ON worker_resource_config_check_sessions (resource_config_check_session_id)
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX worker_resource_config_check_sessions_worker_base_resource_type_id ON worker_resource_config_check_sessions (worker_base_resource_type_id)
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX resources_resource_config_id ON resources (resource_config_id)
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX build_image_resource_caches_resource_cache_id ON build_image_resource_caches (resource_cache_id)
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX volumes_worker_task_cache_id ON volumes (worker_task_cache_id)
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX build_image_resource_caches_build_id ON build_image_resource_caches (build_id)
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX containers_worker_resource_config_check_session_id ON containers (worker_resource_config_check_session_id)
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX pipes_team_id ON pipes (team_id)
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX worker_task_caches_worker_name ON worker_task_caches (worker_name)
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX worker_task_caches_job_id ON worker_task_caches (job_id)
  `)
	if err != nil {
		return err
	}

	return nil
}

func AddBuildsInterceptibleCompletedIndex(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		CREATE INDEX builds_interceptible_completed ON builds (interceptible, completed);
	`)
	if err != nil {
		return err
	}

	return nil
}

func DropUnusedBuildsCompletedIndex(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		DROP INDEX builds_completed
	`)
	if err != nil {
		return err
	}

	return nil
}

func AddUniqueIndexToVolumeHandles(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		CREATE UNIQUE INDEX volumes_handle ON volumes (handle)
	`)
	if err != nil {
		return err
	}

	return nil
}

func AddTeamIdToWorkerResourceConfigCheckSessions(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
	  ALTER TABLE worker_resource_config_check_sessions
		ADD COLUMN team_id integer REFERENCES teams (id) ON DELETE CASCADE
	`)
	if err != nil {
		return err
	}

	return nil
}

func AddViewsForBuildStates(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		CREATE MATERIALIZED VIEW latest_completed_builds_per_job AS
		WITH latest_build_ids_per_job AS (
			SELECT MAX(b.id) AS build_id
			FROM builds b
			INNER JOIN jobs j ON j.id = b.job_id
			WHERE b.status NOT IN ('pending', 'started')
			GROUP BY b.job_id
		)
		SELECT b.*
		FROM builds b
		INNER JOIN latest_build_ids_per_job l ON l.build_id = b.id
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE MATERIALIZED VIEW next_builds_per_job AS
		WITH latest_build_ids_per_job AS (
			SELECT MIN(b.id) AS build_id
			FROM builds b
			INNER JOIN jobs j ON j.id = b.job_id
			WHERE b.status IN ('pending', 'started')
			GROUP BY b.job_id
		)
		SELECT b.*
		FROM builds b
		INNER JOIN latest_build_ids_per_job l ON l.build_id = b.id
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE MATERIALIZED VIEW transition_builds_per_job AS
		WITH builds_before_transition AS (
			SELECT b.job_id, MAX(b.id)
			FROM builds b
			LEFT OUTER JOIN jobs j ON (b.job_id = j.id)
			LEFT OUTER JOIN latest_completed_builds_per_job s ON b.job_id = s.job_id
			WHERE b.status != s.status
			AND b.status NOT IN ('pending', 'started')
			GROUP BY b.job_id
		)
		SELECT DISTINCT ON (b.job_id) b.*
		FROM builds b
		LEFT OUTER JOIN builds_before_transition ON b.job_id = builds_before_transition.job_id
		WHERE builds_before_transition.max IS NULL
		AND b.status NOT IN ('pending', 'started')
		OR b.id > builds_before_transition.max
		ORDER BY b.job_id, b.id ASC
	`)
	if err != nil {
		return err
	}

	return nil
}

type turbineMetadata struct {
	Guid     string `json:"guid"`
	Endpoint string `json:"endpoint"`
}

func AddEngineAndEngineMetadataToBuilds(tx migration.LimitedTx) error {
	_, err := tx.Exec(`ALTER TABLE builds ADD COLUMN engine varchar(16)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`ALTER TABLE builds ADD COLUMN engine_metadata text`)
	if err != nil {
		return err
	}

	cursor := 0

	for {
		var id int
		var guid, endpoint string

		err := tx.QueryRow(`
      SELECT id, guid, endpoint
      FROM builds
      WHERE id > $1
      AND guid != ''
      ORDER BY id ASC
      LIMIT 1
    `, cursor).Scan(&id, &guid, &endpoint)
		if err != nil {
			if err == sql.ErrNoRows {
				break
			}

			return err
		}

		cursor = id

		engineMetadata := turbineMetadata{
			Guid:     guid,
			Endpoint: endpoint,
		}

		payload, err := json.Marshal(engineMetadata)
		if err != nil {
			continue
		}

		_, err = tx.Exec(`
      UPDATE builds
      SET engine = $1, engine_metadata = $2
      WHERE id = $3
    `, "turbine", payload, id)
		if err != nil {
			continue
		}
	}

	_, err = tx.Exec(`ALTER TABLE builds DROP COLUMN guid`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`ALTER TABLE builds DROP COLUMN endpoint`)
	if err != nil {
		return err
	}

	return nil
}

func AddUniqueIndexesToMaterializedViews(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		CREATE UNIQUE INDEX latest_completed_builds_per_job_id ON latest_completed_builds_per_job (id)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE UNIQUE INDEX next_builds_per_job_id ON next_builds_per_job (id)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE UNIQUE INDEX transition_builds_per_job_id ON transition_builds_per_job (id)
	`)
	if err != nil {
		return err
	}

	return nil
}

func AddFailedStateToVolumes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`ALTER TYPE volume_state RENAME TO volume_state_old`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE TYPE volume_state AS ENUM (
			'creating',
			'created',
			'destroying',
			'failed'
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`ALTER TABLE volumes ALTER state DROP DEFAULT,
										DROP CONSTRAINT cannot_invalidate_during_initialization,
										ALTER state SET DATA TYPE volume_state USING state::text::volume_state,
										ALTER parent_state SET DATA TYPE volume_state USING parent_state::text::volume_state`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE volumes
    ADD CONSTRAINT cannot_invalidate_during_initialization CHECK (
    (
      state IN ('created', 'destroying', 'failed') AND (
      (
        worker_resource_cache_id IS NULL
      ) AND (
        worker_base_resource_type_id IS NULL
      ) AND (
        worker_task_cache_id IS NULL
      ) AND (
        container_id IS NULL
      )
    )
      ) OR (
        (
          worker_resource_cache_id IS NOT NULL
        ) OR (
          worker_base_resource_type_id IS NOT NULL
        ) OR (
          worker_task_cache_id IS NOT NULL
        ) OR (
          container_id IS NOT NULL
        )
      )
    )
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`ALTER TABLE volumes ALTER state SET DEFAULT 'creating'`)
	if err != nil {
		return err
	}
	return nil
}

func AddFailedStateToContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`ALTER TYPE container_state RENAME TO container_state_old`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE TYPE container_state AS ENUM (
			'creating',
			'created',
			'destroying',
			'failed'
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`ALTER TABLE containers ALTER state DROP DEFAULT,
										ALTER state SET DATA TYPE container_state USING state::text::container_state
										`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`ALTER TABLE containers ALTER state SET DEFAULT 'creating'`)
	if err != nil {
		return err
	}
	return nil
}

func UseMd5ForResourceCacheVersions(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE resource_caches DROP CONSTRAINT "resource_caches_resource_config_id_version_params_hash_key";
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE UNIQUE INDEX resource_caches_resource_config_id_version_params_hash_key
		ON resource_caches (resource_config_id, md5(version), params_hash);
	`)
	if err != nil {
		return err
	}

	return nil
}

func CreateResourceSpaces(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		CREATE TABLE resource_spaces (
			id serial PRIMARY KEY,
			resource_id int REFERENCES resources (id) ON DELETE CASCADE,
			name text NOT NULL,
			UNIQUE (resource_id, name)
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO resource_spaces(id, resource_id, name) SELECT id, id, 'default' from resources;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE versioned_resources RENAME resource_id TO resource_space_id;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE versioned_resources DROP CONSTRAINT fkey_resource_id;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE versioned_resources ADD CONSTRAINT resource_space_id_fkey FOREIGN KEY (resource_space_id) REFERENCES resource_spaces (id) ON DELETE CASCADE;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER INDEX versioned_resources_resource_id_type_version RENAME TO versioned_resources_resource_space_id_type_version;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER INDEX versioned_resources_resource_id_idx RENAME TO versioned_resources_resource_space_id_idx;
	`)
	return err
}

func AddVersionToBuildEvents(tx migration.LimitedTx) error {
	_, err := tx.Exec(`ALTER TABLE build_events ADD COLUMN version text`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`UPDATE build_events	SET version = '1.0'`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`ALTER TABLE build_events ALTER COLUMN version SET NOT NULL`)
	if err != nil {
		return err
	}

	return nil
}

func AddCompletedToBuilds(tx migration.LimitedTx) error {
	_, err := tx.Exec(`ALTER TABLE builds ADD COLUMN completed boolean NOT NULL default false`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`UPDATE builds SET completed = (status NOT IN ('pending', 'started'))`)
	if err != nil {
		return err
	}

	return nil
}

func AddWorkers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`CREATE TABLE workers (
    addr text NOT NULL,
    expires timestamp NULL,
    active_containers integer DEFAULT 0,
		UNIQUE (addr)
	)`)
	if err != nil {
		return err
	}

	return nil
}

func AddEnabledToBuilds(tx migration.LimitedTx) error {
	_, err := tx.Exec(`ALTER TABLE versioned_resources ADD COLUMN enabled boolean NOT NULL default true`)
	if err != nil {
		return err
	}

	return nil
}

func CreateEventIDSequencesForInFlightBuilds(tx migration.LimitedTx) error {
	cursor := 0

	for {
		var id, eventIDStart int

		err := tx.QueryRow(`
      SELECT id, max(event_id)
      FROM builds
      LEFT JOIN build_events
      ON build_id = id
      WHERE id > $1
      AND status = 'started'
      GROUP BY id
      ORDER BY id ASC
      LIMIT 1
    `, cursor).Scan(&id, &eventIDStart)
		if err != nil {
			if err == sql.ErrNoRows {
				break
			}

			return err
		}

		cursor = id

		_, err = tx.Exec(fmt.Sprintf(`
      CREATE SEQUENCE %s MINVALUE 0 START WITH %d
    `, buildEventSeq(id), eventIDStart+1))
		if err != nil {
			return err
		}
	}

	return nil
}

func buildEventSeq(buildID int) string {
	return fmt.Sprintf("build_event_id_seq_%d", buildID)
}

func AddResourceTypesToWorkers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`ALTER TABLE workers ADD COLUMN resource_types text`)
	if err != nil {
		return err
	}

	return nil
}

func AddPlatformAndTagsToWorkers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`ALTER TABLE workers ADD COLUMN platform text`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`ALTER TABLE workers ADD COLUMN tags text`)
	if err != nil {
		return err
	}

	return nil
}

func AddIdToConfig(tx migration.LimitedTx) error {
	_, err := tx.Exec(`ALTER TABLE config ADD COLUMN id integer NOT NULL DEFAULT 0`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE SEQUENCE config_id_seq`)
	if err != nil {
		return err
	}

	return nil
}

func ConvertJobBuildConfigToJobPlans(tx migration.LimitedTx) error {
	var configPayload []byte

	err := tx.QueryRow(`
    SELECT config
    FROM config
  `).Scan(&configPayload)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}

		return err
	}

	var config internal_26.Config
	err = json.Unmarshal(configPayload, &config)
	if err != nil {
		return err
	}

	for ji, job := range config.Jobs {
		if len(job.Plan) > 0 { // skip jobs already converted to plans
			continue
		}

		convertedSequence := internal_26.PlanSequence{}

		inputAggregates := make(internal_26.PlanSequence, len(job.InputConfigs))
		for ii, input := range job.InputConfigs {
			name := input.RawName
			resource := input.Resource
			if name == "" {
				name = input.Resource
				resource = ""
			}

			inputAggregates[ii] = internal_26.PlanConfig{
				Get:        name,
				Resource:   resource,
				RawTrigger: input.RawTrigger,
				Passed:     input.Passed,
				Params:     input.Params,
			}
		}

		if len(inputAggregates) > 0 {
			convertedSequence = append(convertedSequence, internal_26.PlanConfig{Aggregate: &inputAggregates})
		}

		if job.TaskConfig != nil || job.TaskConfigPath != "" {
			convertedSequence = append(convertedSequence, internal_26.PlanConfig{
				Task:           "build", // default name
				TaskConfigPath: job.TaskConfigPath,
				TaskConfig:     job.TaskConfig,
				Privileged:     job.Privileged,
			})
		}

		outputAggregates := make(internal_26.PlanSequence, len(job.OutputConfigs))
		for oi, output := range job.OutputConfigs {
			var conditions *internal_26.Conditions
			if output.RawPerformOn != nil { // NOT len(0)
				conditionsCasted := internal_26.Conditions(output.RawPerformOn)
				conditions = &conditionsCasted
			}

			outputAggregates[oi] = internal_26.PlanConfig{
				Put:        output.Resource,
				Conditions: conditions,
				Params:     output.Params,
			}
		}

		if len(outputAggregates) > 0 {
			convertedSequence = append(convertedSequence, internal_26.PlanConfig{Aggregate: &outputAggregates})
		}

		// zero-out old-style config so they're omitted from new payload
		config.Jobs[ji].InputConfigs = nil
		config.Jobs[ji].OutputConfigs = nil
		config.Jobs[ji].TaskConfigPath = ""
		config.Jobs[ji].TaskConfig = nil
		config.Jobs[ji].Privileged = false

		config.Jobs[ji].Plan = convertedSequence
	}

	migratedConfig, err := json.Marshal(config)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE config
		SET config = $1, id = nextval('config_id_seq')
  `, migratedConfig)

	return err
}

func AddCheckErrorToResources(tx migration.LimitedTx) error {
	_, err := tx.Exec(`ALTER TABLE resources ADD COLUMN check_error text NULL`)

	return err
}

func AddPausedToResources(tx migration.LimitedTx) error {
	_, err := tx.Exec(`ALTER TABLE resources ADD COLUMN paused boolean DEFAULT(false)`)

	return err
}

func AddPausedToJobs(tx migration.LimitedTx) error {
	_, err := tx.Exec(`ALTER TABLE jobs ADD COLUMN paused boolean DEFAULT(false)`)

	return err
}

func CreateJobsSerialGroups(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		CREATE TABLE jobs_serial_groups (
			id serial PRIMARY KEY,
			job_name text REFERENCES jobs (name),
			serial_group text NOT NULL,
			UNIQUE (job_name, serial_group)
		)
	`)
	return err
}

func CreatePipes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		CREATE TABLE pipes (
			id text PRIMARY KEY,
			url text
		)
	`)
	return err
}

func RenameConfigToPipelines(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
	ALTER TABLE config
    RENAME TO pipelines
	`)
	return err
}

func RenamePipelineIDToVersionAddPrimaryKey(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
	ALTER TABLE pipelines
		RENAME COLUMN id to version
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE pipelines ADD COLUMN id serial PRIMARY KEY;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER SEQUENCE config_id_seq RENAME TO config_version_seq
	`)
	if err != nil {
		return err
	}

	return nil
}

func AddNameToPipelines(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE pipelines ADD COLUMN name text;
	`)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE pipelines ADD CONSTRAINT constraint_pipelines_name_unique UNIQUE (name);
	`)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE pipelines
		SET name = 'main';
	`)

	if err != nil {
		return err
	}

	return err
}

func AddPipelineIDToResources(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE versioned_resources DROP CONSTRAINT versioned_resources_resource_name_fkey;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE resources DROP CONSTRAINT resources_pkey;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE resources ADD COLUMN id serial PRIMARY KEY;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE resources ADD COLUMN pipeline_id int REFERENCES pipelines (id);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE resources
		SET pipeline_id = (
			SELECT id
			FROM pipelines
			WHERE name = 'main'
			LIMIT 1
		);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE resources ADD CONSTRAINT unique_pipeline_id_name UNIQUE (pipeline_id, name);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE resources ALTER COLUMN pipeline_id SET NOT NULL;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE resources ALTER COLUMN name SET NOT NULL;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE versioned_resources ADD COLUMN resource_id int;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE versioned_resources
		SET resource_id = resources.id
		FROM resources
		WHERE versioned_resources.resource_name = resources.name;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE versioned_resources ADD CONSTRAINT fkey_resource_id FOREIGN KEY (resource_id) REFERENCES resources (id);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE versioned_resources DROP COLUMN resource_name;
	`)

	return err

}

func AddPipelineIDToJobs(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE builds DROP CONSTRAINT builds_job_name_fkey
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE jobs_serial_groups DROP CONSTRAINT jobs_serial_groups_job_name_fkey
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE jobs DROP CONSTRAINT jobs_pkey
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE jobs ADD COLUMN id serial PRIMARY KEY
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE jobs ADD COLUMN pipeline_id int REFERENCES pipelines (id)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE jobs
		SET pipeline_id = (
		  SELECT id
		  FROM pipelines
		  WHERE name = 'main'
		  LIMIT 1
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE jobs ADD CONSTRAINT jobs_unique_pipeline_id_name UNIQUE (pipeline_id, name)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE jobs ALTER COLUMN pipeline_id SET NOT NULL
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE jobs ALTER COLUMN name SET NOT NULL
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE builds ADD COLUMN job_id int
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE builds
		SET job_id = jobs.id
		FROM jobs
		WHERE builds.job_name = jobs.name
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE builds ADD CONSTRAINT fkey_job_id FOREIGN KEY (job_id) REFERENCES jobs (id)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE builds DROP COLUMN job_name
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE jobs_serial_groups ADD COLUMN job_id int
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE jobs_serial_groups
		SET job_id = jobs.id
		FROM jobs
		WHERE jobs_serial_groups.job_name = jobs.name
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE jobs_serial_groups ADD CONSTRAINT fkey_job_id FOREIGN KEY (job_id) REFERENCES jobs (id)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE jobs_serial_groups DROP COLUMN job_name
	`)
	return err
}

func AddPausedToPipelines(tx migration.LimitedTx) error {

	_, err := tx.Exec(`
		ALTER TABLE pipelines ADD COLUMN paused boolean DEFAULT(false);
`)

	return err

}

func AddOrderingToPipelines(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE pipelines ADD COLUMN ordering int DEFAULT(0) NOT NULL
	`)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`
			UPDATE pipelines
			SET ordering = id
	`)

	return err
}

func AddInputsDeterminedToBuilds(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE builds ADD COLUMN inputs_determined bool NOT NULL DEFAULT false
	`)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`
			UPDATE builds
			SET inputs_determined = true
	`)

	return err
}

func AddExplicitToBuildOutputs(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE build_outputs ADD COLUMN explicit bool NOT NULL DEFAULT false
	`)

	if err != nil {
		return err
	}

	return nil
}

func AddLastCheckedToResources(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE resources ADD COLUMN last_checked timestamp NOT NULL DEFAULT 'epoch';
	`)

	if err != nil {
		return err
	}

	return nil
}

func AddLastTrackedToBuilds(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE builds ADD COLUMN last_tracked timestamp NOT NULL DEFAULT 'epoch';
	`)

	if err != nil {
		return err
	}

	return nil
}

func AddLastScheduledToPipelines(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE pipelines ADD COLUMN last_scheduled timestamp NOT NULL DEFAULT 'epoch';
	`)

	if err != nil {
		return err
	}

	return nil
}

func AddCheckingToResources(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE resources ADD COLUMN checking bool NOT NULL DEFAULT false;
	`)

	if err != nil {
		return err
	}

	return nil
}

func AddUniqueConstraintToResources(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		WITH distinct_vrs AS (
			SELECT DISTINCT ON (resource_id, type, version) id
			FROM versioned_resources
		), deleted_outputs AS (
			DELETE FROM build_outputs WHERE versioned_resource_id NOT IN (SELECT id FROM distinct_vrs)
		), deleted_inputs AS (
			DELETE FROM build_inputs WHERE versioned_resource_id NOT IN (SELECT id FROM distinct_vrs)
		)
		DELETE FROM versioned_resources WHERE id NOT IN (SELECT id FROM distinct_vrs)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE UNIQUE INDEX versioned_resources_resource_id_type_version
		ON versioned_resources (resource_id, type, version)
	`)
	if err != nil {
		return err
	}

	return nil
}

func RemoveSourceFromVersionedResources(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE versioned_resources DROP COLUMN source
	`)
	if err != nil {
		return err
	}

	return nil
}

func AddIndexesToABunchOfStuff(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		CREATE INDEX build_inputs_build_id_versioned_resource_id ON build_inputs (build_id, versioned_resource_id);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX build_outputs_build_id_versioned_resource_id ON build_outputs (build_id, versioned_resource_id);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX builds_job_id ON builds (job_id);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX jobs_pipeline_id ON jobs (pipeline_id);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX resources_pipeline_id ON resources (pipeline_id);
	`)

	return nil
}

func DropLocks(tx migration.LimitedTx) error {
	_, err := tx.Exec("DROP TABLE locks")
	if err != nil {
		return err
	}

	return nil
}

func AddBaggageclaimURLToWorkers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE workers ADD COLUMN baggageclaim_url text NOT NULL DEFAULT '';
	`)

	if err != nil {
		return err
	}

	return nil
}

func AddContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`CREATE TABLE containers (
    handle text NOT NULL,
		pipeline_name text NOT NULL,
		type text NOT NULL,
		name text NOT NULL,
		build_id integer NOT NULL DEFAULT 0,
    worker_name text NOT NULL,
		expires_at timestamp NOT NULL,
		UNIQUE (handle)
	)`)
	if err != nil {
		return err
	}

	return nil
}

func AddNameToWorkers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE workers ADD COLUMN name text;
	`)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE workers
		SET name = workers.addr;
	`)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE workers ADD CONSTRAINT constraint_workers_name_unique UNIQUE (name);
	`)

	if err != nil {
		return err
	}

	return err
}

func AddLastScheduledToBuilds(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE builds ADD COLUMN last_scheduled timestamp NOT NULL DEFAULT 'epoch';
	`)

	if err != nil {
		return err
	}

	return nil
}

func AddCheckTypeAndCheckSourceToContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers ADD COLUMN check_type text;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE containers ADD COLUMN check_source text;
	`)

	return err
}

func AddStepLocationToContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers ADD COLUMN step_location integer DEFAULT 0;
	`)

	return err
}

func AddVolumesAndCacheInvalidator(tx migration.LimitedTx) error {
	_, err := tx.Exec(`CREATE TABLE volumes (
	  id serial primary key,
    worker_name text NOT NULL,
		expires_at timestamp NOT NULL,
		ttl text null,
		handle text not null,
		resource_version text not null,
		resource_hash text not null,
		UNIQUE (handle)
	)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE TABLE cache_invalidator (
		last_invalidated timestamp NOT NULL DEFAULT 'epoch'
	)`)
	if err != nil {
		return err
	}

	return nil
}

func AddCompositeUniqueConstraintToVolumes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE volumes DROP CONSTRAINT volumes_handle_key;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE volumes ADD UNIQUE (worker_name, handle);
	`)

	return err
}

func AddWorkingDirectoryToContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers ADD COLUMN working_directory text;
	`)

	return err
}

func MakeContainerWorkingDirectoryNotNull(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers ALTER COLUMN working_directory SET DEFAULT '';
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
	  UPDATE containers SET working_directory = '' WHERE working_directory IS null;
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
	  ALTER TABLE containers ALTER COLUMN working_directory SET NOT NULL;
  `)
	return err
}

func AddEnvVariablesToContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers ADD COLUMN env_variables text NOT NULL DEFAULT '[]';
	`)

	return err
}

func AddModifiedTimeToVersionedResourcesAndBuildOutputs(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE versioned_resources
		ADD COLUMN modified_time timestamp NOT NULL DEFAULT now();
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE build_outputs
		ADD COLUMN modified_time timestamp NOT NULL DEFAULT now();
`)
	return err
}

func ReplaceStepLocationWithPlanID(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE containers DROP COLUMN step_location;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE containers ADD COLUMN plan_id text;
	`)

	return err
}

func AddTeamsColumnToPipelinesAndTeamsTable(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    CREATE TABLE teams (
      id serial PRIMARY KEY,
			name text NOT NULL,
      CONSTRAINT constraint_teams_name_unique UNIQUE (name)
    )
  `)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO teams (name) VALUES ('main')
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE pipelines ADD COLUMN team_id integer REFERENCES teams (id);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE pipelines
		SET team_id = sub.id
		FROM (
			SELECT id
			FROM teams
			WHERE name = 'main'
		) AS sub
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE pipelines ALTER COLUMN team_id SET NOT NULL;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX pipelines_team_id ON pipelines (team_id);
	`)
	return err
}

func CascadePipelineDeletes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE build_events DROP CONSTRAINT build_events_build_id_fkey;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE build_events ADD CONSTRAINT build_events_build_id_fkey FOREIGN KEY (build_id) REFERENCES builds (id) ON DELETE CASCADE;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE build_outputs DROP CONSTRAINT build_outputs_build_id_fkey;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE build_outputs ADD CONSTRAINT build_outputs_build_id_fkey FOREIGN KEY (build_id) REFERENCES builds(id) ON DELETE CASCADE;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE build_outputs DROP CONSTRAINT build_outputs_versioned_resource_id_fkey;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE build_outputs ADD CONSTRAINT build_outputs_versioned_resource_id_fkey FOREIGN KEY (versioned_resource_id) REFERENCES versioned_resources(id) ON DELETE CASCADE;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE build_inputs DROP CONSTRAINT build_inputs_build_id_fkey;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE build_inputs ADD CONSTRAINT build_inputs_build_id_fkey FOREIGN KEY (build_id) REFERENCES builds(id) ON DELETE CASCADE;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`

		ALTER TABLE build_inputs DROP CONSTRAINT build_inputs_versioned_resource_id_fkey;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE build_inputs ADD CONSTRAINT build_inputs_versioned_resource_id_fkey FOREIGN KEY (versioned_resource_id) REFERENCES versioned_resources(id) ON DELETE CASCADE;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE jobs_serial_groups DROP CONSTRAINT fkey_job_id;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE jobs_serial_groups ADD CONSTRAINT fkey_job_id FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE builds DROP CONSTRAINT fkey_job_id;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE builds ADD CONSTRAINT fkey_job_id FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE versioned_resources DROP CONSTRAINT fkey_resource_id;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE versioned_resources ADD CONSTRAINT fkey_resource_id FOREIGN KEY (resource_id) REFERENCES resources(id) ON DELETE CASCADE;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE resources DROP CONSTRAINT resources_pipeline_id_fkey;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE resources ADD CONSTRAINT resources_pipeline_id_fkey FOREIGN KEY (pipeline_id) REFERENCES pipelines(id) ON DELETE CASCADE;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE jobs DROP CONSTRAINT jobs_pipeline_id_fkey;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE jobs ADD CONSTRAINT jobs_pipeline_id_fkey FOREIGN KEY (pipeline_id) REFERENCES pipelines (id) ON DELETE CASCADE;
	`)
	return err
}

func AddTeamIDToPipelineNameUniqueness(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE pipelines
		ADD CONSTRAINT pipelines_name_team_id UNIQUE (name, team_id);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE pipelines
		DROP CONSTRAINT constraint_pipelines_name_unique;
	`)
	return err
}

func MakeVolumesExpiresAtNullable(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE volumes
		ALTER COLUMN expires_at DROP NOT NULL;
	`)
	return err
}

func AddAuthFieldsToTeams(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE teams
    ADD COLUMN basic_auth json null;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE teams
    ADD COLUMN github_auth json null;
  `)

	return err
}

func AddAdminToTeams(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
          ALTER TABLE teams
          ADD COLUMN admin bool DEFAULT false;
      `)

	return err
}

func MakeContainersLinkToPipelineIds(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers
		ADD COLUMN pipeline_id INT NULL;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE containers c SET pipeline_id =
		(SELECT id FROM pipelines p WHERE p.name = c.pipeline_name);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE containers
		DROP COLUMN pipeline_name;
	`)
	return err
}

func MakeContainersLinkToResourceIds(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers
		ADD COLUMN resource_id INT NULL;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE containers c SET resource_id =
		(SELECT id from resources r where r.name = c.name 
													AND r.pipeline_id = c.pipeline_id
													AND c.type = 'check');
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE containers
		RENAME COLUMN name TO step_name;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE containers
		SET step_name = ''
		WHERE resource_id IS NOT NULL;
	`)
	return err
}

func MakeContainersBuildIdsNullable(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers
		ALTER COLUMN build_id DROP NOT NULL;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE containers SET build_id = NULL
		WHERE build_id = 0;
	`)
	return err
}

func MakeContainersLinkToWorkerIds(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE workers
		ADD COLUMN id BIGSERIAL PRIMARY KEY;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE containers
		ADD COLUMN worker_id INT NULL;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE containers c SET worker_id =
		(select id from workers w where w.name = c.worker_name);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE containers
		DROP COLUMN worker_name;
	`)
	return err
}

func RemoveVolumesWithExpiredWorkers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
	DELETE FROM volumes v
	WHERE (SELECT COUNT(name) FROM workers w WHERE w.name = v.worker_name) = 0;
	`)
	return err
}

func AddWorkerIDToVolumes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE volumes ADD COLUMN worker_id integer;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE volumes v set worker_id =
		(SELECT id from workers w where w.name = v.worker_name);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE volumes ALTER COLUMN worker_id SET NOT NULL;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX volumes_worker_id ON volumes (worker_id);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE volumes DROP CONSTRAINT volumes_worker_name_handle_key;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE volumes ADD UNIQUE (worker_id, handle);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE volumes DROP COLUMN worker_name;
	`)
	return err
}

func RemoveWorkerIds(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE volumes ADD COLUMN worker_name text;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE volumes v set worker_name =
		(SELECT name from workers w where w.id = v.worker_id);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		DELETE FROM volumes WHERE worker_name IS NULL;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE volumes ALTER COLUMN worker_name SET NOT NULL;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX volumes_worker_name ON volumes (worker_name);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE volumes DROP CONSTRAINT volumes_worker_id_handle_key;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE volumes ADD UNIQUE (worker_name, handle);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE volumes DROP COLUMN worker_id;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE containers
		ADD COLUMN worker_name TEXT NULL;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE containers c SET worker_name =
		(select name from workers w where w.id = c.worker_id);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE containers
		DROP COLUMN worker_id;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE workers
		DROP COLUMN id;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE workers
		ADD PRIMARY KEY (name);
	`)
	return err
}

func AddAttemptsToContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers ADD COLUMN attempts text NULL;
	`)
	return err
}

func AddStageToContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    CREATE TYPE container_stage AS ENUM (
      'check',
      'get',
      'run'
    )
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE containers ADD COLUMN stage container_stage NOT NULL DEFAULT 'run';
	`)
	return err
}

func AddImageResourceVersions(tx migration.LimitedTx) error {
	_, err := tx.Exec(`CREATE TABLE image_resource_versions (
    id serial PRIMARY KEY,
    version text NOT NULL,
    build_id integer REFERENCES builds (id) ON DELETE CASCADE NOT NULL,
    plan_id text NOT NULL,
    resource_hash text NOT NULL,
		UNIQUE (build_id, plan_id)
	)`)
	return err
}

func MakeContainerIdentifiersUnique(tx migration.LimitedTx) error {
	// This migration used to run the following, which led to errors from
	// postgres as the resulting data for maintaining the index was too large:

	// _, err := tx.Exec(`
	// 	ALTER TABLE containers ADD UNIQUE
	// 	(worker_name, resource_id, check_type, check_source, build_id, plan_id, stage)
	// `)
	// return err

	// The error was:
	//
	//   index row size 3528 exceeds maximum 2712 for index
	//   "containers_worker_name_resource_id_check_type_check_source__key"`

	return nil
}

func CleanUpMassiveUniqueConstraint(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers
		DROP CONSTRAINT IF EXISTS containers_worker_name_resource_id_check_type_check_source__key
	`)
	return err
}

func AddPipelineBuildEventsTables(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE build_events
		DROP CONSTRAINT build_events_build_id_fkey
	`)
	if err != nil {
		return fmt.Errorf("failed to update build events foreign key: %s", err)
	}

	rows, err := tx.Query(`SELECT id FROM pipelines`)
	if err != nil {
		return err
	}

	defer rows.Close()

	var pipelineIDs []int

	for rows.Next() {
		var pipelineID int
		err = rows.Scan(&pipelineID)
		if err != nil {
			return fmt.Errorf("failed to scan pipeline ID: %s", err)
		}

		pipelineIDs = append(pipelineIDs, pipelineID)
	}

	for _, pipelineID := range pipelineIDs {
		err = createBuildEventsTable(tx, pipelineID)
		if err != nil {
			return fmt.Errorf("failed to create build events table: %s", err)
		}

		err = populateBuildEventsTable(tx, pipelineID)
		if err != nil {
			return fmt.Errorf("failed to populate build events: %s", err)
		}
	}

	_, err = tx.Exec(`
		CREATE INDEX build_events_build_id_idx ON build_events (build_id)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX build_outputs_build_id_idx ON build_outputs (build_id)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX build_inputs_build_id_idx ON build_inputs (build_id)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX build_outputs_versioned_resource_id_idx ON build_outputs (versioned_resource_id)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX build_inputs_versioned_resource_id_idx ON build_inputs (versioned_resource_id)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX image_resource_versions_build_id_idx ON image_resource_versions (build_id)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX pipelines_team_id_idx ON pipelines (team_id)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX resources_pipeline_id_idx ON resources (pipeline_id)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX jobs_pipeline_id_idx ON jobs (pipeline_id)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX jobs_serial_groups_job_id_idx ON jobs_serial_groups (job_id)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX builds_job_id_idx ON builds (job_id)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX versioned_resources_resource_id_idx ON versioned_resources (resource_id)
	`)
	if err != nil {
		return fmt.Errorf("failed to create indexes: %s", err)
	}

	_, err = tx.Exec(`
		DELETE FROM ONLY build_events
		WHERE build_id IN (SELECT id FROM builds WHERE job_id IS NOT NULL)
	`)
	if err != nil {
		return fmt.Errorf("failed to clean up build events: %s", err)
	}

	return nil
}

func createBuildEventsTable(tx migration.LimitedTx, pipelineID int) error {
	_, err := tx.Exec(fmt.Sprintf(`
		CREATE TABLE pipeline_build_events_%[1]d ()
		INHERITS (build_events)
	`, pipelineID))
	if err != nil {
		return err
	}

	_, err = tx.Exec(fmt.Sprintf(`
		CREATE INDEX pipelines_build_events_%[1]d_build_id ON pipeline_build_events_%[1]d (build_id)
	`, pipelineID))
	if err != nil {
		return err
	}

	_, err = tx.Exec(fmt.Sprintf(`
		CREATE UNIQUE INDEX pipeline_build_events_%[1]d_build_id_event_id ON pipeline_build_events_%[1]d USING btree (build_id, event_id)
	`, pipelineID))
	if err != nil {
		return err
	}

	return nil
}

func populateBuildEventsTable(tx migration.LimitedTx, pipelineID int) error {
	_, err := tx.Exec(fmt.Sprintf(`
		INSERT INTO pipeline_build_events_%[1]d (
			build_id, type, payload, event_id, version
		)
		SELECT build_id, type, payload, event_id, version
		FROM build_events AS e, builds AS b, jobs AS j
		WHERE j.pipeline_id = $1
		AND b.job_id = j.id
		AND b.id = e.build_id
	`, pipelineID), pipelineID)
	if err != nil {
		return fmt.Errorf("failed to insert: %s", err)
	}

	return err
}

func AddBuildPreparation(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
	CREATE TABLE build_preparation (
		build_id integer PRIMARY KEY REFERENCES builds (id) ON DELETE CASCADE,
		paused_pipeline text DEFAULT 'unknown',
		paused_job text DEFAULT 'unknown',
		max_running_builds text DEFAULT 'unknown',
		inputs json DEFAULT '{}',
		completed bool DEFAULT false
	)
	`)
	return err
}

func DropCompletedFromBuildPreparation(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
	ALTER TABLE build_preparation
	DROP COLUMN completed
	`)
	return err
}

func AddInputsSatisfiedToBuildPreparation(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
	ALTER TABLE build_preparation
	ADD COLUMN inputs_satisfied text NOT NULL DEFAULT 'unknown'
	`)
	return err
}

func AddOrderToVersionedResources(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE versioned_resources 
		ADD COLUMN check_order int
		DEFAULT 0 NOT NULL;
	`)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE versioned_resources
		SET check_order = id;
	`)

	return err
}

func AddImageResourceTypeAndSourceToContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers ADD COLUMN image_resource_type text
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE containers ADD COLUMN image_resource_source text
	`)
	return err
}

func AddUserToContainer(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
	  ALTER TABLE containers
		ADD COLUMN process_user text DEFAULT '';
	`)
	return err
}

func ResetPendingBuilds(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
	UPDATE builds
	SET scheduled = false
	WHERE scheduled = true AND status = 'pending'
	`)
	return err
}

func ResetCheckOrder(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
	UPDATE versioned_resources
	SET check_order = id
	WHERE check_order != id
	`)
	return err
}

func AddTTLToContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers ADD COLUMN ttl text NOT NULL DEFAULT 0;
	`)
	if err != nil {
		return err
	}

	return nil
}

func AddOriginalVolumeHandleToVolumes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE volumes ADD COLUMN original_volume_handle text DEFAULT null;
	`)
	if err != nil {
		return err
	}

	return nil
}

func DropNotNullResourceConstraintsOnVolumes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE volumes ALTER COLUMN resource_version DROP NOT NULL
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE volumes ALTER COLUMN resource_hash DROP NOT NULL
	`)
	if err != nil {
		return err
	}

	return nil
}

func AddOutputNameToVolumes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE volumes ADD COLUMN output_name text DEFAULT null;
	`)
	if err != nil {
		return err
	}

	return nil
}

func CreateResourceTypes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		CREATE TABLE resource_types (
			id serial PRIMARY KEY,
			pipeline_id int REFERENCES pipelines (id) ON DELETE CASCADE,
			name text NOT NULL,
			type text NOT NULL,
			version text,
			UNIQUE (pipeline_id, name)
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE containers
		ADD COLUMN resource_type_version text
	`)
	return err
}

func AddLastCheckedAndCheckingToResourceTypes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE resource_types
		ADD COLUMN last_checked timestamp NOT NULL DEFAULT 'epoch',
		ADD COLUMN checking bool NOT NULL DEFAULT false
	`)
	return err
}

func AddHttpProxyHttpsProxyNoProxyToWorkers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE workers
		ADD COLUMN http_proxy_url text,
		ADD COLUMN https_proxy_url text,
		ADD COLUMN no_proxy text
	`)
	return err
}

func AddPathToVolumes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE volumes
		ADD COLUMN path text
	`)
	return err
}

func AddModifiedTimeToBuildInputs(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE build_inputs
		ADD COLUMN modified_time timestamp NOT NULL DEFAULT now();
`)
	return err
}

func AddHostPathVersionToVolumes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE volumes
		ADD COLUMN host_path_version text;
`)
	return err
}

func AddBestIfUsedByToContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers
		ADD COLUMN best_if_used_by timestamp;
`)
	return err
}

func AddStartTimeToWorkers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE workers
		ADD COLUMN start_time integer;
`)
	return err
}
