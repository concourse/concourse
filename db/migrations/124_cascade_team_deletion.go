package migrations

import (
	"fmt"

	"github.com/concourse/atc/db/migration"
)

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
