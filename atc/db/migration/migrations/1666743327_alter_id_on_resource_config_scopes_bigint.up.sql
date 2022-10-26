ALTER TABLE resource_config_scopes ALTER COLUMN id TYPE bigint;

ALTER TABLE resource_config_versions ALTER COLUMN resource_config_scope_id TYPE bigint;

ALTER TABLE resources ALTER COLUMN resource_config_scope_id TYPE bigint;
