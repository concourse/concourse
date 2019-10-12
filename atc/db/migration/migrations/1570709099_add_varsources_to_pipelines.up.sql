BEGIN;
  ALTER TABLE pipelines ADD COLUMN var_sources text,
                        ADD COLUMN var_sources_nonce text;
COMMIT;