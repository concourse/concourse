BEGIN;
  DROP INDEX build_pipes_to_build_id_idx;

  DROP INDEX build_resource_config_version_inputs_version_md5_idx;
COMMIT;
