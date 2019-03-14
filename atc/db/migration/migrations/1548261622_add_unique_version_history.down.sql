BEGIN;
  ALTER TABLE resource_configs
    DROP COLUMN unique_versions_resource_id,
    ADD CONSTRAINT resource_configs_resource_cache_id_so_key UNIQUE (resource_cache_id, source_hash),
    ADD CONSTRAINT resource_configs_base_resource_type_id_so_key UNIQUE (base_resource_type_id, source_hash);

  ALTER TABLE base_resource_types
    DROP COLUMN unique_version_history;
COMMIT;
