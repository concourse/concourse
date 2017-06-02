package migrations

import "github.com/concourse/atc/db/migration"

func CreateJobsSerialGroups(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		CREATE TABLE jobs_serial_groups (
			id serial PRIMARY KEY,
			job_name text REFERENCES jobs (name),
			serial_group text NOT NULL,
			UNIQUE (job_name, serial_group)
		)
	`)
	return err
}
