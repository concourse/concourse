package migrations

import "github.com/concourse/atc/db/migration"

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
