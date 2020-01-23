BEGIN;
  CREATE TABLE successful_build_outputs (
      "build_id" integer NOT NULL REFERENCES builds (id) ON DELETE CASCADE,
      "job_id" integer NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
      "outputs" jsonb NOT NULL
  );

  CREATE UNIQUE INDEX successful_build_outputs_build_id_idx ON successful_build_outputs (build_id);

  CREATE INDEX on successful_build_outputs USING GIN (outputs jsonb_path_ops) WITH (FASTUPDATE = false);

  CREATE INDEX successful_build_outputs_job_id_idx ON successful_build_outputs (job_id);
COMMIT;
