DROP INDEX containers_in_memory_check_build_id_idx;

ALTER TABLE containers
    DROP COLUMN in_memory_check_build_id;
