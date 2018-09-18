BEGIN;
  DROP INDEX resource_caches_resource_config_id_version_params_hash_key;

  ALTER TABLE ONLY resource_caches ADD CONSTRAINT resource_caches_resource_config_id_version_params_hash_key UNIQUE (resource_config_id, version, params_hash);
COMMIT;
