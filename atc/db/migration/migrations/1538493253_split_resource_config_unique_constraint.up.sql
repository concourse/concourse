BEGIN;
  ALTER TABLE ONLY resource_configs
      DROP CONSTRAINT resource_configs_resource_cache_id_base_resource_type_id_so_key,
      ADD CONSTRAINT resource_configs_resource_cache_id_so_key UNIQUE (resource_cache_id, source_hash),
      ADD CONSTRAINT resource_configs_base_resource_type_id_so_key UNIQUE (base_resource_type_id, source_hash);
COMMIT;
