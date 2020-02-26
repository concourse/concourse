BEGIN;
  CREATE INDEX build_pipes_to_build_id_idx ON build_pipes (to_build_id);

  -- Removed due to the index no longer being used
  -- CREATE INDEX build_resource_config_version_inputs_version_md5_idx ON build_resource_config_version_inputs (version_md5);
COMMIT;
