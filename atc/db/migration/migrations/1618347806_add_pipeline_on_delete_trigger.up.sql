CREATE OR REPLACE FUNCTION on_pipeline_delete() RETURNS TRIGGER AS $$
BEGIN
        EXECUTE format('INSERT INTO deleted_pipelines VALUES (%s)', OLD.id);
        RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER deleted_pipelines_insert_trigger AFTER DELETE on pipelines FOR EACH ROW EXECUTE PROCEDURE on_pipeline_delete();
