BEGIN;
  CREATE INDEX resource_config_scope_check_order_idx ON resource_config_versions (resource_config_scope_id, check_order desc);
COMMIT;
