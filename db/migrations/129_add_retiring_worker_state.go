package migrations

import "github.com/BurntSushi/migration"

func AddRetiringWorkerState(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TYPE worker_state
    ADD VALUE 'landed' AFTER 'landing'
    ;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TYPE worker_state
    ADD VALUE 'retiring' AFTER 'landed'
    ;
  `)
	if err != nil {
		return err
	}

	return nil
}
