BEGIN;
  CREATE TABLE worker_artifacts (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    build_id INTEGER NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL
  );

  ALTER TABLE worker_artifacts
    ADD CONSTRAINT worker_artifacts_build_id_fkey FOREIGN KEY (build_id) REFERENCES builds(id) ON DELETE SET NULL;

  ALTER TABLE volumes
    ADD COLUMN worker_artifact_id INTEGER;

  ALTER TABLE volumes
    ADD CONSTRAINT volumes_worker_artifact_id_fkey FOREIGN KEY (worker_artifact_id) REFERENCES worker_artifacts(id) ON DELETE SET NULL;

COMMIT;
