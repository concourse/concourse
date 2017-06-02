package migrations

import "github.com/concourse/atc/db/migration"

func AddLocks(tx migration.LimitedTx) error {
	_, err := tx.Exec(`CREATE TABLE locks (
      id serial PRIMARY KEY,
      name text NOT NULL,
			UNIQUE (name)
	)`)
	if err != nil {
		return err
	}

	return nil
}
