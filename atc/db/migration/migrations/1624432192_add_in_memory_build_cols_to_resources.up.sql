-- ALTER TABLE resources
--     ADD COLUMN in_memory_check_build_id int8,
--     ADD COLUMN in_memory_check_build_start_time timestamptz,
--     ADD COLUMN in_memory_check_build_end_time timestamptz,
--     ADD COLUMN in_memory_check_build_status text,
--     ADD COLUMN in_memory_check_build_plan text;

-- TODO: add an unique index on in_memory_check_build_id

ALTER TABLE resource_config_scopes
    ADD COLUMN last_check_build_id int8,
    ADD COLUMN last_check_build_plan json NULL DEFAULT '{}'::json;

ALTER TABLE containers
    ADD COLUMN in_memory_check_build_id int8;

-- TODO:
--  1. maybe add an index on in_memory_check_build_id
--  2. update the down script