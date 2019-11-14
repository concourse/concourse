BEGIN;
  CREATE TABLE job_pipes (
      job_id integer REFERENCES jobs(id) ON DELETE CASCADE NOT NULL,
      resource_id integer REFERENCES resources(id) ON DELETE CASCADE NOT NULL,
      passed_job_id integer REFERENCES jobs(id) ON DELETE CASCADE
  );
COMMIT;
