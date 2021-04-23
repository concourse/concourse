
ALTER TABLE resource_config_scopes
    ADD COLUMN last_check_succeeded boolean DEFAULT false NOT NULL;