BEGIN;

  ALTER TABLE resource_configs
    ADD COLUMN "default_space" text,
    ADD COLUMN "last_checked" timestamp with time zone NOT NULL DEFAULT '1970-01-01 00:00:00',
    ADD COLUMN "last_check_finished" timestamp with time zone NOT NULL DEFAULT '1970-01-01 00:00:00',
    ADD COLUMN "check_error" text;

  ALTER TABLE resources
    DROP COLUMN resource_config_scope_id;

  ALTER TABLE spaces
    ADD COLUMN resource_config_id integer REFERENCES resource_configs (id) ON DELETE CASCADE,
    DROP COLUMN resource_config_scope_id;

  ALTER TABLE resource_config_versions
    DROP COLUMN resource_config_scope_id;

  CREATE UNIQUE INDEX spaces_resource_config_id_name_key ON spaces (resource_config_id, name);

  DROP TABLE resource_config_scopes;

COMMIT;
