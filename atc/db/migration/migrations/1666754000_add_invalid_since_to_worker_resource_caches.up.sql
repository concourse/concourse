-- Add invalid_since timestamp that will be set to when worker_base_resource_type_id is to null,
-- so that the cache can be GC-ed after certain period.
ALTER TABLE worker_resource_caches
    ADD COLUMN invalid_since timestamp with time zone NULL;

UPDATE worker_resource_caches
    SET invalid_since = now()
    WHERE worker_base_resource_type_id IS NULL;

DROP INDEX worker_resource_caches_uniq;
-- The unique index should be only applied on valid caches.
CREATE UNIQUE INDEX worker_resource_caches_uniq ON worker_resource_caches USING btree (worker_name, resource_cache_id) WHERE worker_base_resource_type_id IS NOT NULL;

-- This function will be called by trigger to set invalid_since to current time.
CREATE FUNCTION worker_resource_caches_fkey_trigger_function() RETURNS TRIGGER LANGUAGE plpgsql AS
$$
begin
    UPDATE worker_resource_caches
    SET invalid_since = now()
    WHERE id = NEW.id;
    RETURN NULL;
end;
$$;

-- This triggered will fired when set worker_base_resource_type_id to null.
CREATE TRIGGER worker_resource_caches_fkey_trigger
    AFTER UPDATE ON worker_resource_caches
    FOR EACH ROW
    WHEN (OLD.worker_base_resource_type_id IS NOT NULL AND NEW.worker_base_resource_type_id IS NULL)
EXECUTE PROCEDURE worker_resource_caches_fkey_trigger_function();
