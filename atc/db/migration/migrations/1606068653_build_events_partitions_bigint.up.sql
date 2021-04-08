
-- migrate each pipeline partition
DO $$
DECLARE
  pipeline record;
BEGIN
FOR pipeline IN
  SELECT id, name FROM pipelines
LOOP
  RAISE NOTICE 'renaming old indexes for pipeline % (%)', pipeline.id, pipeline.name;
  EXECUTE format('DROP INDEX pipeline_build_events_%s_build_id', pipeline.id);
  EXECUTE format('ALTER INDEX pipeline_build_events_%s_build_id_event_id RENAME TO pipeline_build_events_%s_build_id_old_event_id', pipeline.id, pipeline.id);

  RAISE NOTICE 'creating new indexes for pipeline % (%)', pipeline.id, pipeline.name;
  EXECUTE format('CREATE UNIQUE INDEX pipeline_build_events_%s_build_id_event_id ON pipeline_build_events_%s (build_id, event_id)', pipeline.id, pipeline.id);
END LOOP;
END;
$$ LANGUAGE plpgsql;

-- backfill indexes for each team partition (these are for one-off builds)
DO $$
DECLARE
  team record;
BEGIN
FOR team IN
  SELECT id, name FROM teams
LOOP
  RAISE NOTICE 'creating new indexes for team % (%)', team.id, team.name;
  EXECUTE format('CREATE UNIQUE INDEX team_build_events_%s_build_id_event_id ON team_build_events_%s (build_id, event_id)', team.id, team.id);
END LOOP;
END;
$$ LANGUAGE plpgsql;

-- set up both old and new indexes on pipeline creation
--
-- this is really just to maintain consistency with old and new pipelines
CREATE OR REPLACE FUNCTION on_pipeline_insert() RETURNS TRIGGER AS $$
BEGIN
  EXECUTE format('CREATE TABLE IF NOT EXISTS pipeline_build_events_%s () INHERITS (build_events)', NEW.id);
  EXECUTE format('CREATE UNIQUE INDEX pipeline_build_events_%s_build_id_event_id ON pipeline_build_events_%s (build_id, event_id)', NEW.id, NEW.id);
  EXECUTE format('CREATE UNIQUE INDEX pipeline_build_events_%s_build_id_old_event_id ON pipeline_build_events_%s (build_id_old, event_id)', NEW.id, NEW.id);
  RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- set up indexes on team creation
--
-- don't bother setting up old indexes since this never existed before
CREATE OR REPLACE FUNCTION on_team_insert() RETURNS TRIGGER AS $$
BEGIN
  EXECUTE format('CREATE TABLE IF NOT EXISTS team_build_events_%s () INHERITS (build_events)', NEW.id);
  EXECUTE format('CREATE UNIQUE INDEX team_build_events_%s_build_id_event_id ON team_build_events_%s (build_id, event_id)', NEW.id, NEW.id);
  RETURN NULL;
END;
$$ LANGUAGE plpgsql;
