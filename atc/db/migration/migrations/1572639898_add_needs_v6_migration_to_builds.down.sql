BEGIN;
  ALTER TABLE builds
    DROP COLUMN needs_v6_migration;

  DROP INDEX needs_v6_migration_idx;
COMMIT;
