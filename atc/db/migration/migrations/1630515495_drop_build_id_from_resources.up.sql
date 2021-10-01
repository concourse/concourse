DROP INDEX resources_build_id_idx;
ALTER TABLE resources
    DROP COLUMN build_id,
    ADD COLUMN in_memory_build_id bigint,
    ADD COLUMN in_memory_build_start_time timestamp with time zone,
    ADD COLUMN in_memory_build_plan json NULL;
CREATE INDEX resources_in_memory_build_id_idx ON resources (in_memory_build_id);
