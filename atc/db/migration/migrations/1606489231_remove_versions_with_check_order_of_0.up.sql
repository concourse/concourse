BEGIN;

  DELETE FROM resource_config_versions WHERE check_order = 0;

COMMIT;
