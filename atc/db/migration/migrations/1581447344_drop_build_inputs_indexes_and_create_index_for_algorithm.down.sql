BEGIN;
  CREATE INDEX build_resource_config_version_inputs_resource_id_idx ON build_resource_config_version_inputs (resource_id);
  CREATE INDEX build_resource_config_version_inputs_build_id_idx ON build_resource_config_version_inputs (build_id);

  DROP INDEX build_inputs_resource_versions_idx;
COMMIT;
