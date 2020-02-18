BEGIN;
  ALTER TABLE builds
    ADD COLUMN needs_v6_migration boolean NOT NULL DEFAULT true;

  CREATE INDEX needs_v6_migration_idx ON builds (job_id, COALESCE(rerun_of, id) DESC, id DESC) WHERE status = 'succeeded' AND needs_v6_migration;
COMMIT;
