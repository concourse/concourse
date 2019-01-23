BEGIN;
  ALTER TABLE resource_configs
    ADD COLUMN unique_versions_resource_id integer,
    ADD CONSTRAINT resource_configs_unique_versions_resource_id_fkey FOREIGN KEY (unique_versions_resource_id) REFERENCES resources(id) ON DELETE CASCADE,
    DROP CONSTRAINT resource_configs_resource_cache_id_so_key,
    DROP CONSTRAINT resource_configs_base_resource_type_id_so_key,
    ADD CONSTRAINT resource_configs_resource_cache_id_so_unique_versions_resource_id_key UNIQUE (resource_cache_id, source_hash, unique_versions_resource_id),
    ADD CONSTRAINT resource_configs_base_resource_type_id_so_unique_versions_resource_id_key UNIQUE (base_resource_type_id, source_hash, unique_versions_resource_id);

  ALTER TABLE base_resource_types
    ADD COLUMN unique_version_history boolean NOT NULL DEFAULT false;
COMMIT;
