BEGIN;
  CREATE INDEX resources_build_id_idx ON resources (build_id);
COMMIT;
