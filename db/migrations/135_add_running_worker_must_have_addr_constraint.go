package migrations

import "github.com/BurntSushi/migration"

func AddRunningWorkerMustHaveAddrConstraint(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE workers
    ADD CONSTRAINT addr_when_running CHECK (
			state != 'stalled' AND addr IS NOT NULL
		)
	`)
	if err != nil {
		return err
	}

	return nil
}
