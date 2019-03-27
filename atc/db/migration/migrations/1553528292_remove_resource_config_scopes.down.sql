BEGIN
  CREATE TABLE resource_config_scopes (
    "id" serial NOT NULL PRIMARY KEY,
    "resource_config_id" integer NOT NULL REFERENCES resource_configs (id) ON DELETE CASCADE,
    "resource_id" integer REFERENCES resources (id) ON DELETE CASCADE,
    "last_checked" timestamp with time zone NOT NULL DEFAULT '1970-01-01 00:00:00',
    "check_error" text,
    "default_space" text,
    "last_check_finished" timestamp with time zone NOT NULL DEFAULT '1970-01-01 00:00:00';
  );

  CREATE UNIQUE INDEX resource_config_scopes_resource_id_resource_config_id_uniq
  ON resource_config_scopes (resource_id, resource_config_id)
  WHERE resource_id IS NOT NULL;

  CREATE UNIQUE INDEX resource_config_scopes_resource_config_id_uniq
  ON resource_config_scopes (resource_config_id)
  WHERE resource_id IS NULL;

  ALTER TABLE resource_configs
    DROP COLUMN default_space,
    DROP COLUMN last_checked,
    DROP COLUMN last_check_finished,
    DROP COLUMN check_error;

  ALTER TABLE resources
    ADD COLUMN resource_config_scope_id integer REFERENCES resource_config_scopes (id) ON DELETE SET NULL;

  ALTER TABLE spaces
    ADD COLUMN resource_config_scope_id integer REFERENCES resource_config_scopes (id) ON DELETE CASCADE,
    DROP COLUMN resource_config_id;

  ALTER TABLE resource_config_versions
    ADD COLUMN resource_config_scope_id integer NOT NULL REFERENCES resource_config_scopes (id) ON DELETE CASCADE;

  CREATE UNIQUE INDEX spaces_resource_config_scope_id_name_key ON spaces (resource_config_scope_id, name);
COMMIT;
