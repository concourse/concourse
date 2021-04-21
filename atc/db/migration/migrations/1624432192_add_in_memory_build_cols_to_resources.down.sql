-- ALTER TABLE resources
--     DROP COLUMN in_memory_check_build_id,
--     DROP COLUMN in_memory_check_build_start_time,
--     DROP COLUMN in_memory_check_build_end_time,
--     DROP COLUMN in_memory_check_build_status,
--     DROP COLUMN in_memory_check_build_plan;

ALTER TABLE resource_config_scopes
    DROP COLUMN last_check_build_id,
    DROP COLUMN last_check_build_plan;