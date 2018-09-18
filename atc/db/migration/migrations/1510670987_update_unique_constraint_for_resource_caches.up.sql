BEGIN;
  ALTER TABLE resource_caches DROP CONSTRAINT resource_caches_resource_config_id_version_params_hash_key;

  CREATE UNIQUE INDEX resource_caches_resource_config_id_version_params_hash_key ON resource_caches (resource_config_id, md5(version), params_hash);
COMMIT;
