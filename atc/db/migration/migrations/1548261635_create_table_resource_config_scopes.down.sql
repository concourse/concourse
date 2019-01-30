BEGIN;

  ALTER TABLE resources
    DROP COLUMN resource_config_scope_id;

  TRUNCATE TABLE resource_config_versions CASCADE;

  ALTER TABLE resource_config_versions
    DROP COLUMN resource_config_scope_id,
    ADD COLUMN "resource_config_id" integer NOT NULL REFERENCES resource_configs (id) ON DELETE CASCADE,
    ADD CONSTRAINT "resource_config_id_and_version_md5_unique" UNIQUE ("resource_config_id", "version_md5");

  DROP TABLE resource_config_scopes;

  DROP INDEX resource_configs_resource_cache_id_so_key;
  DROP INDEX resource_configs_base_resource_type_id_so_key;

  ALTER TABLE resource_configs
    ADD COLUMN last_checked timestamp with time zone NOT NULL DEFAULT '1970-01-01 00:00:00',
    ADD COLUMN unique_versions_resource_id integer,
    ADD COLUMN check_error text,
    ADD CONSTRAINT resource_configs_unique_versions_resource_id_fkey FOREIGN KEY (unique_versions_resource_id) REFERENCES resources(id) ON DELETE CASCADE,
    ADD CONSTRAINT resource_configs_resource_cache_id_so_unique_versions_resource_id_key UNIQUE (resource_cache_id, source_hash, unique_versions_resource_id),
    ADD CONSTRAINT resource_configs_base_resource_type_id_so_unique_versions_resource_id_key UNIQUE (base_resource_type_id, source_hash, unique_versions_resource_id);

COMMIT;
