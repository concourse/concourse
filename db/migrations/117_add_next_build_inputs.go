package migrations

import "github.com/concourse/atc/db/migration"

func AddNextBuildInputs(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		CREATE TABLE independent_build_inputs (
			id serial PRIMARY KEY,
			job_id integer NOT NULL,
			CONSTRAINT independent_build_inputs_job_id_fkey
				FOREIGN KEY (job_id)
				REFERENCES jobs (id)
				ON DELETE CASCADE,
			input_name text NOT NULL,
			CONSTRAINT independent_build_inputs_unique_job_id_input_name
				UNIQUE (job_id, input_name),
			version_id integer NOT NULL,
			CONSTRAINT independent_build_inputs_version_id_fkey
				FOREIGN KEY (version_id)
				REFERENCES versioned_resources (id)
				ON DELETE CASCADE,
			first_occurrence bool NOT NULL
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE TABLE next_build_inputs (
			id serial PRIMARY KEY,
			job_id integer NOT NULL,
			CONSTRAINT next_build_inputs_job_id_fkey
				FOREIGN KEY (job_id)
				REFERENCES jobs (id)
				ON DELETE CASCADE,
			input_name text NOT NULL,
			CONSTRAINT next_build_inputs_unique_job_id_input_name
				UNIQUE (job_id, input_name),
			version_id integer NOT NULL,
			CONSTRAINT next_build_inputs_version_id_fkey
				FOREIGN KEY (version_id)
				REFERENCES versioned_resources (id)
				ON DELETE CASCADE,
			first_occurrence bool NOT NULL
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE jobs
			ADD COLUMN resource_check_finished_at timestamp NOT NULL DEFAULT 'epoch',
			ADD COLUMN resource_check_waiver_end integer NOT NULL DEFAULT 0,
			ADD COLUMN inputs_determined bool NOT NULL DEFAULT false,
			ADD COLUMN max_in_flight_reached bool NOT NULL DEFAULT false
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE builds
			DROP COLUMN inputs_determined,
			DROP COLUMN last_scheduled
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		DROP TABLE build_preparation
	`)
	return err
}
