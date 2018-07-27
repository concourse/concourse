BEGIN;
  DROP TRIGGER IF EXISTS team_build_events_delete_trigger ON teams;
  DROP TRIGGER IF EXISTS team_build_events_insert_trigger ON teams;

  DROP FUNCTION IF EXISTS on_team_delete();
  DROP FUNCTION IF EXISTS on_team_insert();
COMMIT;

