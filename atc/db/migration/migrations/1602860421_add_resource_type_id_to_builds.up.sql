BEGIN;
  ALTER TABLE builds
  ADD COLUMN resource_type_id integer REFERENCES resource_types (id) ON DELETE CASCADE;
COMMIT;
