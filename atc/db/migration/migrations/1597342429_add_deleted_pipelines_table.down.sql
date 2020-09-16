BEGIN;
    DROP TABLE IF EXISTS deleted_pipelines;

    CREATE OR REPLACE FUNCTION on_pipeline_delete() RETURNS TRIGGER AS $$
    BEGIN
        EXECUTE format('DROP TABLE IF EXISTS pipeline_build_events_%s', OLD.id);
        RETURN NULL;
    END;
    $$ LANGUAGE plpgsql;

    DROP TRIGGER IF EXISTS pipeline_build_events_delete_trigger ON pipelines;
    CREATE TRIGGER pipeline_build_events_delete_trigger AFTER DELETE on pipelines FOR EACH ROW EXECUTE PROCEDURE on_pipeline_delete();
COMMIT;
