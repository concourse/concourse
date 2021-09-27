DROP INDEX containers_in_memory_build_id_idx;

ALTER TABLE containers
    DROP COLUMN in_memory_build_id,
    DROP COLUMN in_memory_build_create_time;
