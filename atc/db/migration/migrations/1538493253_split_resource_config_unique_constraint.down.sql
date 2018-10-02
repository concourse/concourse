BEGIN;
  ALTER TABLE ONLY resource_configs
      ADD CONSTRAINT resource_configs_resource_cache_id_base_resource_type_id_so_key UNIQUE (resource_cache_id, base_resource_type_id, source_hash),
      DROP CONSTRAINT resource_configs_resource_cache_id_so_key,
      DROP CONSTRAINT resource_configs_base_resource_type_id_so_key;
COMMIT;
