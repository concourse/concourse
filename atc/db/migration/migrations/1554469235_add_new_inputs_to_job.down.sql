BEGIN;
  ALTER TABLE jobs DROP COLUMN has_new_inputs;
COMMIT;
