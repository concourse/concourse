BEGIN;
  CREATE INDEX builds_resource_type_id_idx ON builds (resource_type_id);
COMMIT;
