BEGIN;

  CREATE INDEX resource_config_versions_version ON resource_config_versions USING gin(version jsonb_path_ops) WITH (FASTUPDATE = false);

COMMIT;
