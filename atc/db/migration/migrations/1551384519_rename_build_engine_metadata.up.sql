BEGIN;
  DROP MATERIALIZED VIEW transition_builds_per_job;
  DROP MATERIALIZED VIEW next_builds_per_job;
  DROP MATERIALIZED VIEW latest_completed_builds_per_job;

  ALTER TABLE builds RENAME COLUMN engine TO schema;
  ALTER TABLE builds RENAME COLUMN engine_metadata TO private_plan;
COMMIT;
