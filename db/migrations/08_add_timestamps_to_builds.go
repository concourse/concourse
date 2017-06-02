package migrations

import "github.com/concourse/atc/db/migration"

func AddTimestampsToBuilds(tx migration.LimitedTx) error {
	_, err := tx.Exec(`ALTER TABLE builds ADD COLUMN start_time timestamp with time zone`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`ALTER TABLE builds ADD COLUMN end_time timestamp with time zone`)
	if err != nil {
		return err
	}

	return nil
}
