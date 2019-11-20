BEGIN;
  ALTER TABLE pipelines DROP COLUMN var_sources,
                        DROP COLUMN nonce;
COMMIT;
