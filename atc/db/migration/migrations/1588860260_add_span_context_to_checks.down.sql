BEGIN;
  ALTER TABLE checks DROP COLUMN span_context;
COMMIT;
