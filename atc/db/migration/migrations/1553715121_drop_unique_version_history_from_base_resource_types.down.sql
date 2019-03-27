BEGIN;
  ALTER TABLE base_resource_types
    ADD COLUMN unique_version_history boolean NOT NULL DEFAULT false;
COMMIT;
