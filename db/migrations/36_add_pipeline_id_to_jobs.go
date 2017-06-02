package migrations

import "github.com/concourse/atc/db/migration"

func AddPipelineIDToJobs(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE builds DROP CONSTRAINT builds_job_name_fkey
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE jobs_serial_groups DROP CONSTRAINT jobs_serial_groups_job_name_fkey
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE jobs DROP CONSTRAINT jobs_pkey
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE jobs ADD COLUMN id serial PRIMARY KEY
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE jobs ADD COLUMN pipeline_id int REFERENCES pipelines (id)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE jobs
		SET pipeline_id = (
		  SELECT id
		  FROM pipelines
		  WHERE name = 'main'
		  LIMIT 1
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE jobs ADD CONSTRAINT jobs_unique_pipeline_id_name UNIQUE (pipeline_id, name)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE jobs ALTER COLUMN pipeline_id SET NOT NULL
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE jobs ALTER COLUMN name SET NOT NULL
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE builds ADD COLUMN job_id int
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE builds
		SET job_id = jobs.id
		FROM jobs
		WHERE builds.job_name = jobs.name
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE builds ADD CONSTRAINT fkey_job_id FOREIGN KEY (job_id) REFERENCES jobs (id)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE builds DROP COLUMN job_name
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE jobs_serial_groups ADD COLUMN job_id int
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE jobs_serial_groups
		SET job_id = jobs.id
		FROM jobs
		WHERE jobs_serial_groups.job_name = jobs.name
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE jobs_serial_groups ADD CONSTRAINT fkey_job_id FOREIGN KEY (job_id) REFERENCES jobs (id)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE jobs_serial_groups DROP COLUMN job_name
	`)
	return err
}
