BEGIN;
  CREATE INDEX containers_build_id_not_null ON containers (worker_name) WHERE build_id IS NOT NULL;
COMMIT;
