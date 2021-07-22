ALTER TABLE resources ADD COLUMN build_id bigint REFERENCES builds (id) ON DELETE SET NULL;
CREATE INDEX resources_build_id_idx ON resources (build_id);

DROP INDEX resource_config_scopes_last_check_build_id_idx;

ALTER TABLE resource_config_scopes
    DROP COLUMN last_check_build_id,
    DROP COLUMN last_check_build_plan;

DROP INDEX containers_in_memory_check_build_id_idx;

ALTER TABLE containers
    DROP COLUMN in_memory_check_build_id;
