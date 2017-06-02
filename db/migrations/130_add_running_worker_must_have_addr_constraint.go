package migrations

import "github.com/concourse/atc/db/migration"

func AddRunningWorkerMustHaveAddrConstraint(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE workers
		ALTER COLUMN baggageclaim_url DROP NOT NULL
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE workers
    ADD CONSTRAINT addr_when_running CHECK (
			(
				state != 'stalled' AND addr IS NOT NULL AND baggageclaim_url IS NOT NULL
			) OR (
				state = 'stalled' AND addr IS NULL AND baggageclaim_url IS NULL
			)
		)
	`)
	if err != nil {
		return err
	}

	return nil
}
