
CREATE OR REPLACE FUNCTION clear_resource_config_ids() RETURNS TRIGGER AS $$
BEGIN
        EXECUTE format('UPDATE resources SET resource_config_id = NULL, resource_config_scope_id = NULL WHERE id=%s', NEW.id);
        RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS resources_config_update_clears_config_ids_trigger ON resources;
CREATE TRIGGER resources_config_update_clears_config_ids_trigger
	AFTER UPDATE on resources
	FOR EACH ROW
	WHEN ((NEW.config IS DISTINCT FROM OLD.config) AND (NEW.active IS TRUE))
	EXECUTE PROCEDURE clear_resource_config_ids();
