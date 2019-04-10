BEGIN;
  DROP INDEX build_resource_config_version_inputs_resource_id_idx;
  DROP INDEX build_resource_config_version_inputs_build_id_idx;

  DROP INDEX build_resource_config_version_outputs_resource_id_idx;
  DROP INDEX build_resource_config_version_outputs_build_id_idx;
COMMIT;
