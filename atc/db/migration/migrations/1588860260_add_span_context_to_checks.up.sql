BEGIN;
  ALTER TABLE checks ADD COLUMN span_context jsonb;
COMMIT;
