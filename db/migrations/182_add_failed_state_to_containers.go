package migrations

import "github.com/concourse/atc/db/migration"

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
