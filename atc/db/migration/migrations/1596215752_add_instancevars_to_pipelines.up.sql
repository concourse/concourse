BEGIN;

  ALTER TABLE pipelines ADD COLUMN instance_vars jsonb;

  ALTER TABLE pipelines DROP CONSTRAINT pipelines_name_team_id;

  CREATE UNIQUE INDEX pipelines_name_team_id
  ON pipelines (name, team_id)
  WHERE instance_vars IS NULL;

  CREATE UNIQUE INDEX pipelines_name_team_id_instance_vars
  ON pipelines (name, team_id, instance_vars)
  WHERE instance_vars IS NOT NULL;

  ALTER TABLE containers ADD COLUMN meta_pipeline_instance_vars text DEFAULT ''::text NOT NULL;

COMMIT;
