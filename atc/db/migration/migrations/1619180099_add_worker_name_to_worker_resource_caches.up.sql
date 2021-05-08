ALTER TABLE worker_resource_caches
    ADD COLUMN worker_name text REFERENCES workers(name) ON DELETE CASCADE;

DROP INDEX worker_resource_caches_uniq;

CREATE UNIQUE INDEX worker_resource_caches_uniq
    ON worker_resource_caches (worker_name, resource_cache_id, worker_base_resource_type_id);

-- populate worker-name for existing caches with current worker name
UPDATE worker_resource_caches wrc
    SET worker_name = (SELECT worker_name FROM worker_base_resource_types wbrt WHERE wbrt.id = wrc.worker_base_resource_type_id)
    WHERE worker_name is null;
