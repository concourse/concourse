BEGIN;
  ALTER TABLE resources
    ADD COLUMN api_pinned_version jsonb;
COMMIT;
