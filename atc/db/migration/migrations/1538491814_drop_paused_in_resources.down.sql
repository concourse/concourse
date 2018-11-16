BEGIN;
  ALTER TABLE resources
    ADD COLUMN paused boolean DEFAULT false,
COMMIT;
