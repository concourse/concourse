BEGIN;
  ALTER TABLE volumes
    DROP CONSTRAINT volumes_worker_artifact_id_fkey;

  ALTER TABLE volumes
    DROP COLUMN worker_artifact_id;

  DROP TABLE worker_artifacts;
COMMIT;
