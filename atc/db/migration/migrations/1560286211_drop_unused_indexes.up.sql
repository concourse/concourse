BEGIN;
  DROP INDEX builds_team_id;
  DROP INDEX builds_pipeline_id;
  DROP INDEX builds_name;
  DROP INDEX build_resource_config_version_inputs_resource_id_idx;
  DROP INDEX builds_job_id;
COMMIT;
