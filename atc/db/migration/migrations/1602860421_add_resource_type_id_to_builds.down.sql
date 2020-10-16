BEGIN;
  ALTER TABLE builds
  DROP COLUMN resource_type_id;
COMMIT;
