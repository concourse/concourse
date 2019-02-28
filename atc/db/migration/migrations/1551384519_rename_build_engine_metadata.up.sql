BEGIN;
  ALTER TABLE builds RENAME COLUMN engine TO schema;
  ALTER TABLE builds RENAME COLUMN engine_metadata TO private_plan;
COMMIT;
