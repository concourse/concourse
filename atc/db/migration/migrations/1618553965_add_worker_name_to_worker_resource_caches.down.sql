ALTER TABLE worker_resource_caches
    DROP COLUMN worker_name;

DROP INDEX worker_resource_caches_uniq;

CREATE UNIQUE INDEX worker_resource_caches_uniq
    ON worker_resource_caches (resource_cache_id, worker_base_resource_type_id);
