BEGIN;
  ALTER TABLE resources ADD COLUMN check_error text;
  ALTER TABLE resource_types ADD COLUMN check_error text;
  ALTER TABLE resource_config_scopes ADD COLUMN check_error text;
COMMIT;
