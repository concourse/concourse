package migrations

import "github.com/concourse/atc/db/migration"

func AddBaggageclaimURLToWorkers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE workers ADD COLUMN baggageclaim_url text NOT NULL DEFAULT '';
	`)

	if err != nil {
		return err
	}

	return nil
}
