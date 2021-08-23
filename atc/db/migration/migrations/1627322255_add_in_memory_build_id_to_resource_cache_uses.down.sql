DROP INDEX resource_cache_uses_in_memory_build_id_idx;

ALTER TABLE resource_cache_uses
    DROP COLUMN in_memory_build_id,
    DROP COLUMN in_memory_build_create_time;
