BEGIN;
  CREATE INDEX builds_pipeline_id ON builds USING btree (pipeline_id);
  CREATE INDEX builds_team_id ON builds USING btree (team_id);
COMMIT;
