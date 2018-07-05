BEGIN;
  CREATE OR REPLACE FUNCTION on_team_delete() RETURNS TRIGGER AS $$
  BEGIN
          EXECUTE format('DROP TABLE IF EXISTS team_build_events_%s', OLD.id);
          RETURN NULL;
  END;
  $$ LANGUAGE plpgsql;


  CREATE OR REPLACE FUNCTION on_team_insert() RETURNS TRIGGER AS $$
  BEGIN
          EXECUTE format('CREATE TABLE IF NOT EXISTS team_build_events_%s () INHERITS (build_events)', NEW.id);
          RETURN NULL;
  END;
  $$ LANGUAGE plpgsql;


  DROP TRIGGER IF EXISTS team_build_events_delete_trigger ON teams;
  CREATE TRIGGER team_build_events_delete_trigger AFTER DELETE on teams FOR EACH ROW EXECUTE PROCEDURE on_team_delete();

  DROP TRIGGER IF EXISTS team_build_events_insert_trigger ON teams;
  CREATE TRIGGER team_build_events_insert_trigger AFTER INSERT on teams FOR EACH ROW EXECUTE PROCEDURE on_team_insert();
COMMIT;

