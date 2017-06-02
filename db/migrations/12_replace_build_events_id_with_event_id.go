package migrations

import "github.com/concourse/atc/db/migration"

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
