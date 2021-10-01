ALTER TABLE resources
    ADD COLUMN build_id bigint REFERENCES builds (id) ON DELETE SET NULL,
    DROP COLUMN in_memory_build_id,
    DROP COLUMN in_memory_build_start_time,
    DROP COLUMN in_memory_build_plan;
CREATE INDEX resources_build_id_idx ON resources (build_id);
