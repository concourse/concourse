package migrations

import "github.com/concourse/atc/db/migration"

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
