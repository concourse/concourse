BEGIN;

  ALTER TABLE pipelines DROP COLUMN instance_vars;

  ALTER TABLE ONLY pipelines ADD CONSTRAINT pipelines_name_team_id UNIQUE (name, team_id);

  DROP INDEX pipelines_name_team_id;

  DROP INDEX pipelines_name_team_id_instance_vars;

COMMIT;
