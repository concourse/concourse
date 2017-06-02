package migrations

import "github.com/concourse/atc/db/migration"

func AddContainers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`CREATE TABLE containers (
    handle text NOT NULL,
		pipeline_name text NOT NULL,
		type text NOT NULL,
		name text NOT NULL,
		build_id integer NOT NULL DEFAULT 0,
    worker_name text NOT NULL,
		expires_at timestamp NOT NULL,
		UNIQUE (handle)
	)`)
	if err != nil {
		return err
	}

	return nil
}
