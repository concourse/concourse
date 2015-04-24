package migrations

import "github.com/BurntSushi/migration"

func AddPipelineIDToJobs(tx migration.LimitedTx) error {

	_, err := tx.Exec(`
		ALTER TABLE builds DROP CONSTRAINT builds_job_name_fkey;

		ALTER TABLE jobs_serial_groups DROP CONSTRAINT jobs_serial_groups_job_name_fkey;

		ALTER TABLE jobs DROP CONSTRAINT jobs_pkey;

		ALTER TABLE jobs ADD COLUMN id serial PRIMARY KEY;

		ALTER TABLE jobs ADD COLUMN pipeline_id int REFERENCES pipelines (id);

		UPDATE jobs
		SET pipeline_id = (
		  SELECT id
		  FROM pipelines
		  WHERE name = 'main'
		  LIMIT 1
		);

		ALTER TABLE jobs ADD CONSTRAINT jobs_unique_pipeline_id_name UNIQUE (pipeline_id, name);

		ALTER TABLE jobs ALTER COLUMN pipeline_id SET NOT NULL;
		ALTER TABLE jobs ALTER COLUMN name SET NOT NULL;


		ALTER TABLE builds ADD COLUMN job_id int;

		UPDATE builds
		SET job_id = jobs.id
		FROM jobs
		WHERE builds.job_name = jobs.name;


		ALTER TABLE builds ADD CONSTRAINT fkey_job_id FOREIGN KEY (job_id) REFERENCES jobs (id);

		ALTER TABLE builds DROP COLUMN job_name;


		ALTER TABLE jobs_serial_groups ADD COLUMN job_id int;

		UPDATE jobs_serial_groups
		SET job_id = jobs.id
		FROM jobs
		WHERE jobs_serial_groups.job_name = jobs.name;

		ALTER TABLE jobs_serial_groups ADD CONSTRAINT fkey_job_id FOREIGN KEY (job_id) REFERENCES jobs (id);

		ALTER TABLE jobs_serial_groups DROP COLUMN job_name;
`)

	return err

}
