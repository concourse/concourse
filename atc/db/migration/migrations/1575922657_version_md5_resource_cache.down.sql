BEGIN;
  DROP INDEX resource_caches_resource_config_id_version_md5_params_hash_uniq

  ALTER TABLE resource_caches DROP COLUMN version_md5;

  COMMENT ON COLUMN resource_caches.version IS NULL;

  CREATE UNIQUE INDEX resource_caches_resource_config_id_version_params_hash_uniq
  ON resource_caches (resource_config_id, md5(version::text), params_hash);
COMMIT;
