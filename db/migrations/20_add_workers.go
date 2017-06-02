package migrations

import "github.com/concourse/atc/db/migration"

func AddWorkers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`CREATE TABLE workers (
    addr text NOT NULL,
    expires timestamp NULL,
    active_containers integer DEFAULT 0,
		UNIQUE (addr)
	)`)
	if err != nil {
		return err
	}

	return nil
}
