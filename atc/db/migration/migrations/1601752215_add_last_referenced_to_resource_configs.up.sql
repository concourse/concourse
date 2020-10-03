BEGIN;
  ALTER TABLE resource_configs
  ADD COLUMN last_referenced timestamp with time zone NOT NULL DEFAULT '1970-01-01 00:00:00'::timestamp with time zone;
COMMIT;
