BEGIN;
  ALTER TABLE base_resource_types
    DROP COLUMN unique_version_history;
COMMIT;
