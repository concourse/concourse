BEGIN;
  CREATE TABLE job_pipes (
      job_id integer REFERENCES jobs(id) ON DELETE CASCADE NOT NULL,
      resource_id integer REFERENCES resources(id) ON DELETE CASCADE NOT NULL,
      passed_job_id integer REFERENCES jobs(id) ON DELETE CASCADE
  );

  CREATE INDEX job_pipes_resource_id_idx ON job_pipes (resource_id);

  CREATE INDEX job_pipes_passed_job_id_idx ON job_pipes (passed_job_id);

  CREATE INDEX job_pipes_job_id_idx ON job_pipes (job_id);
COMMIT;
