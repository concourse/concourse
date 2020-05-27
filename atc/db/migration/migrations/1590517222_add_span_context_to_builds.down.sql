BEGIN;
  ALTER TABLE builds DROP COLUMN span_context;
COMMIT;
