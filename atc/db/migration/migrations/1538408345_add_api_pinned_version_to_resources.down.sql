BEGIN;
  ALTER TABLE resources
    DROP COLUMN api_pinned_version;
COMMIT;
