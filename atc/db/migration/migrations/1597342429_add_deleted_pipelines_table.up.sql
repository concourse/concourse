BEGIN;
    DROP TRIGGER IF EXISTS pipeline_build_events_delete_trigger ON pipelines;
    DROP FUNCTION IF EXISTS on_pipeline_delete();

	CREATE TABLE deleted_pipelines (
        id integer NOT NULL,
        deleted_at timestamp without time zone DEFAULT now() NOT NULL
	);
COMMIT;
