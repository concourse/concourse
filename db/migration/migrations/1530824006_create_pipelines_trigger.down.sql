BEGIN;
  DROP TRIGGER IF EXISTS pipeline_build_events_delete_trigger ON pipelines;
  DROP TRIGGER IF EXISTS pipeline_build_events_insert_trigger ON pipelines;

  DROP FUNCTION IF EXISTS on_pipeline_delete();
  DROP FUNCTION IF EXISTS on_pipeline_insert();
COMMIT;
