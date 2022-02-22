ALTER TABLE worker_resource_caches
    DROP CONSTRAINT worker_resource_caches_worker_base_resource_type_id_fkey;

ALTER TABLE worker_resource_caches
    ADD CONSTRAINT worker_resource_caches_worker_base_resource_type_id_fkey
        FOREIGN KEY (worker_base_resource_type_id) REFERENCES worker_base_resource_types(id)
            ON DELETE CASCADE;
