package migrations

import "github.com/BurntSushi/migration"

func ReplaceBuildEventsIDWithEventID(tx migration.LimitedTx) error {
	_, err := tx.Exec(`ALTER TABLE build_events ADD COLUMN event_id integer`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE build_events AS a
		SET event_id = a.id - (
			SELECT b.id
			FROM build_events b
			WHERE b.build_id = a.build_id
			ORDER BY b.id ASC
			LIMIT 1
		)
	`)
	if err != nil {
		return err
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
