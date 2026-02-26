ALTER TABLE resource_config_scopes
  ADD COLUMN next_check_time timestamp with time zone NOT NULL DEFAULT '1970-01-01 00:00:00';
