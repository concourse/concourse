BEGIN;
  ALTER TABLE builds
    DROP COLUMN needs_v6_migration;
COMMIT;
