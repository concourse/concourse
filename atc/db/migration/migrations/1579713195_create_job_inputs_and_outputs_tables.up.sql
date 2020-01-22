BEGIN;
  CREATE TABLE job_inputs (
      name text NOT NULL,
      job_id integer REFERENCES jobs(id) ON DELETE CASCADE NOT NULL,
      resource_id integer REFERENCES resources(id) ON DELETE CASCADE NOT NULL,
      passed_job_id integer REFERENCES jobs(id) ON DELETE CASCADE,
      trigger bool NOT NULL DEFAULT false,
      version text
  );

  CREATE INDEX job_inputs_resource_id_idx ON job_inputs (resource_id);
  CREATE INDEX job_inputs_passed_job_id_idx ON job_inputs (passed_job_id);
  CREATE INDEX job_inputs_job_id_idx ON job_inputs (job_id);

  CREATE TABLE job_outputs (
      name text NOT NULL,
      job_id integer REFERENCES jobs(id) ON DELETE CASCADE NOT NULL,
      resource_id integer REFERENCES resources(id) ON DELETE CASCADE NOT NULL
  );

  CREATE INDEX job_outputs_job_id_idx ON job_outputs (job_id);
  CREATE INDEX job_outputs_resource_id_idx ON job_outputs (resource_id);
COMMIT;
