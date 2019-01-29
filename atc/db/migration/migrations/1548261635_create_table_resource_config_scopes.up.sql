BEGIN;

  CREATE TABLE resource_config_scopes (
    "id" serial NOT NULL PRIMARY KEY,
    "resource_config_id" integer NOT NULL REFERENCES resource_configs (id) ON DELETE CASCADE,
    "resource_id" integer REFERENCES resources (id) ON DELETE CASCADE,
    "last_checked" timestamp with time zone NOT NULL DEFAULT '1970-01-01 00:00:00',
    "check_error" text
  );

  CREATE UNIQUE INDEX resource_config_scopes_resource_id_resource_config_id_uniq
  ON resource_config_scopes (resource_id, resource_config_id)
  WHERE resource_id IS NOT NULL;

  CREATE UNIQUE INDEX resource_config_scopes_resource_config_id_uniq
  ON resource_config_scopes (resource_config_id)
  WHERE resource_id IS NULL;

  ALTER TABLE resource_configs
    DROP COLUMN unique_versions_resource_id,
    DROP COLUMN last_checked,
    DROP COLUMN check_error;

  CREATE UNIQUE INDEX resource_configs_resource_cache_id_so_key
  ON resource_configs (resource_cache_id, source_hash);

  CREATE UNIQUE INDEX resource_configs_base_resource_type_id_so_key
  ON resource_configs (base_resource_type_id, source_hash);

  TRUNCATE TABLE resource_config_versions CASCADE;

  ALTER TABLE resource_config_versions
    DROP COLUMN resource_config_id,
    ADD COLUMN resource_config_scope_id integer NOT NULL REFERENCES resource_config_scopes (id) ON DELETE CASCADE,
    ADD CONSTRAINT "resource_config_scope_id_and_version_md5_unique" UNIQUE ("resource_config_scope_id", "version_md5");

  ALTER TABLE resources
    ADD COLUMN resource_config_scope_id integer REFERENCES resource_config_scopes (id) ON DELETE SET NULL;

COMMIT;
