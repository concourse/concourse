BEGIN;
  ALTER TABLE builds ADD COLUMN span_context jsonb;
COMMIT;
