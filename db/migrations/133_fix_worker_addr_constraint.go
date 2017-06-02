package migrations

import "github.com/concourse/atc/db/migration"

func FixWorkerAddrConstraint(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE workers
		DROP CONSTRAINT addr_when_running
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE workers SET baggageclaim_url = NULL, addr = NULL WHERE state = 'landed'
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE workers
		ADD CONSTRAINT addr_when_running CHECK (
			(
				(state != 'stalled' AND state != 'landed') AND (addr IS NOT NULL OR baggageclaim_url IS NOT NULL)
			) OR (
				(state = 'stalled' OR state = 'landed') AND addr IS NULL AND baggageclaim_url IS NULL
			)
		)
	`)
	if err != nil {
		return err
	}

	return nil
}
