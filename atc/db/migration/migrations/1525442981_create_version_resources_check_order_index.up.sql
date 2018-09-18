BEGIN;
  CREATE INDEX versioned_resources_check_order ON versioned_resources (check_order DESC);
COMMIT;
