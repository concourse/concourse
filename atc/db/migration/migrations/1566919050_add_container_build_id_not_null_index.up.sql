BEGIN;
  CREATE INDEX containers_build_id_not_null ON containers (build_id) WHERE build_id IS NOT NULL;
COMMIT;
