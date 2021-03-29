
CREATE OR REPLACE FUNCTION on_team_insert() RETURNS TRIGGER AS $$
BEGIN
  EXECUTE format('CREATE TABLE IF NOT EXISTS team_build_events_%s () INHERITS (build_events)', NEW.id);
  RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION on_pipeline_insert() RETURNS TRIGGER AS $$
BEGIN
  EXECUTE format('CREATE TABLE IF NOT EXISTS pipeline_build_events_%s () INHERITS (build_events)', NEW.id);
  EXECUTE format('CREATE INDEX pipeline_build_events_%s_build_id ON pipeline_build_events_%s (build_id)', NEW.id, NEW.id);
  EXECUTE format('CREATE UNIQUE INDEX pipeline_build_events_%s_build_id_event_id ON pipeline_build_events_%s (build_id, event_id)', NEW.id, NEW.id);
  RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DO $$
DECLARE
  team record;
BEGIN
FOR team IN
  SELECT id, name FROM teams
LOOP
  RAISE NOTICE 'creating new indexes for team % (%)', team.id, team.name;
  EXECUTE format('DROP INDEX team_build_events_%s_build_id_event_id', team.id);
END LOOP;
END
$$ LANGUAGE plpgsql;

DO $$
DECLARE
  pipeline record;
BEGIN
FOR pipeline IN
  SELECT id, name FROM pipelines
LOOP
  RAISE NOTICE 'dropping new indexes for pipeline % (%)', pipeline.id, pipeline.name;
  EXECUTE format('DROP INDEX pipeline_build_events_%s_build_id_event_id', pipeline.id);

  RAISE NOTICE 'renaming old indexes for pipeline % (%)', pipeline.id, pipeline.name;
  EXECUTE format('CREATE INDEX pipeline_build_events_%s_build_id ON pipeline_build_events_%s (build_id_old)', pipeline.id, pipeline.id);
  EXECUTE format('ALTER INDEX pipeline_build_events_%s_build_id_old_event_id RENAME TO pipeline_build_events_%s_build_id_event_id', pipeline.id, pipeline.id);
END LOOP;
END
$$ LANGUAGE plpgsql;

INSERT INTO migrations_history (version, tstamp, direction, status, dirty) VALUES (1606068653, current_timestamp, 'down', 'passed', false)

