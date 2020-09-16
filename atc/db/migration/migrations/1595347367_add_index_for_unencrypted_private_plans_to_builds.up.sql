BEGIN;
  CREATE INDEX unencrypted_private_plans_build_idx ON builds (id) WHERE nonce IS NULL AND private_plan IS NOT NULL;
COMMIT;
