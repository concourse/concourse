BEGIN;
  CREATE TABLE worker_artifacts (
    id SERIAL PRIMARY KEY,
    path TEXT NOT NULL,
    checksum TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL
  );

  ALTER TABLE volumes
    ADD COLUMN worker_artifact_id integer;

  ALTER TABLE volumes
    ADD CONSTRAINT volumes_worker_artifact_id_fkey FOREIGN KEY (worker_artifact_id) REFERENCES worker_artifacts(id) ON DELETE SET NULL;

COMMIT;
