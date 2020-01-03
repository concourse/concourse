BEGIN;
  CREATE TABLE build_pipes (
    "from_build_id" integer NOT NULL REFERENCES builds (id) ON DELETE CASCADE,
    "to_build_id" integer NOT NULL REFERENCES builds (id) ON DELETE CASCADE
  );

  CREATE UNIQUE INDEX build_pipes_from_build_id_to_build_id_uniq
  ON build_pipes (from_build_id, to_build_id);

  CREATE TABLE next_build_pipes (
    "from_build_id" integer NOT NULL REFERENCES builds (id) ON DELETE CASCADE,
    "to_job_id" integer NOT NULL REFERENCES jobs (id) ON DELETE CASCADE
  );

  CREATE UNIQUE INDEX next_build_pipes_from_build_id_to_job_id_uniq
  ON next_build_pipes (from_build_id, to_job_id);
COMMIT;
