ALTER TABLE worker_resource_caches
    ADD COLUMN worker_name text REFERENCES workers(name) ON DELETE CASCADE;

DROP INDEX worker_resource_caches_uniq;

CREATE UNIQUE INDEX worker_resource_caches_uniq
    ON worker_resource_caches (resource_cache_id, worker_base_resource_type_id, worker_name);
