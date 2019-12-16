BEGIN;
  ALTER TABLE resource_caches ADD COLUMN version text;

  UPDATE resource_caches rc
  SET version = rcv.version
  LEFT JOIN resource_configs ON resource_configs.id = rc.resource_config_id
  FROM resource_config_versions rcv
  WHERE rcv.version_md5 = rc.version_md5;

  DROP INDEX resource_caches_resource_config_id_version_md5_params_hash_uniq

  ALTER TABLE resource_caches DROP COLUMN version_md5;

  CREATE UNIQUE INDEX resource_caches_resource_config_id_version_params_hash_uniq
  ON resource_caches (resource_config_id, md5(version::text), params_hash);
COMMIT;
