BEGIN;
  DROP INDEX IF EXISTS build_resource_config_version_inputs_version_md5_idx;
  DROP INDEX build_resource_config_version_inputs_resource_id_idx;
  DROP INDEX build_resource_config_version_inputs_build_id_idx;

  CREATE INDEX build_inputs_resource_versions_idx ON build_resource_config_version_inputs (resource_id, version_md5);
COMMIT;
