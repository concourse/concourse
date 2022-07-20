DROP TRIGGER worker_resource_caches_fkey_trigger on worker_resource_caches;

DROP FUNCTION worker_resource_caches_fkey_trigger_function();

DELETE FROM worker_resource_caches WHERE worker_base_resource_type_id IS NULL;

DROP INDEX worker_resource_caches_uniq;

CREATE UNIQUE INDEX worker_resource_caches_uniq ON worker_resource_caches USING btree (worker_name, resource_cache_id);

ALTER TABLE worker_resource_caches
    DROP COLUMN invalid_since;