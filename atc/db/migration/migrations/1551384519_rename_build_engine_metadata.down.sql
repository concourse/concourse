BEGIN;
  ALTER TABLE builds RENAME COLUMN schema TO engine;
  ALTER TABLE builds RENAME COLUMN private_plan TO engine_metadata;
COMMIT;
