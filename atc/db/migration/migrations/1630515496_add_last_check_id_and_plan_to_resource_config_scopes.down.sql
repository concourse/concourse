DROP INDEX resource_config_scopes_last_check_build_id_idx;

ALTER TABLE resource_config_scopes
    DROP COLUMN last_check_build_id,
    DROP COLUMN last_check_build_plan;
