package migrations

import (
	"fmt"

	"github.com/concourse/atc/db/migration"
)

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
