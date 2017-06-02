package migrations

import "github.com/concourse/atc/db/migration"

func AddFirstLoggedBuildIDToJobsAndReapTimeToBuildsAndLeases(tx migration.LimitedTx) error {
	_, err := tx.Exec(`ALTER TABLE jobs ADD COLUMN first_logged_build_id int NOT NULL DEFAULT 0`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`ALTER TABLE builds ADD COLUMN reap_time timestamp with time zone`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE TABLE leases (
    id serial PRIMARY KEY,
		name text NOT NULL,
		last_invalidated timestamp NOT NULL DEFAULT 'epoch',
    CONSTRAINT constraint_leases_name_unique UNIQUE (name)
	)`)

	return err
}
