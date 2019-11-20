BEGIN;
  ALTER TABLE pipelines ADD COLUMN var_sources text,
                        ADD COLUMN nonce text;
COMMIT;
