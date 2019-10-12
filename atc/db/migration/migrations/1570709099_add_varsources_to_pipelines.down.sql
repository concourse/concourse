BEGIN;
  ALTER TABLE pipelines DROP COLUMN var_sources,
                        DROP COLUMN var_sources_nonce;
COMMIT;
