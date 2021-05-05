-- delete streamed cache tuples
DELETE FROM worker_resource_caches
    WHERE id IN (
        SELECT wrc.id
            FROM worker_resource_caches wrc
            LEFT JOIN worker_base_resource_types wbrt ON wrc.worker_base_resource_type_id = wbrt.id
            WHERE wrc.worker_name != wbrt.worker_name);

DROP INDEX worker_resource_caches_uniq;

ALTER TABLE worker_resource_caches
    DROP COLUMN worker_name;

CREATE UNIQUE INDEX worker_resource_caches_uniq
    ON worker_resource_caches (resource_cache_id, worker_base_resource_type_id);
