BEGIN;
  ALTER TABLE builds
  DROP COLUMN resource_id;
COMMIT;
