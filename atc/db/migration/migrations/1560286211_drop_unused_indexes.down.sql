BEGIN;
  CREATE INDEX builds_pipeline_id ON builds USING btree (pipeline_id);
  CREATE INDEX builds_team_id ON builds USING btree (team_id);
  CREATE INDEX builds_name ON builds USING btree (name);
  CREATE INDEX build_resource_config_version_inputs_resource_id_idx ON build_resource_config_version_inputs (resource_id);
  CREATE INDEX builds_job_id ON builds USING btree (job_id);
COMMIT;
