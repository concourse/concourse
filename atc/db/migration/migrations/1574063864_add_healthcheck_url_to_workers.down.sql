BEGIN;
  ALTER TABLE workers
    DROP COLUMN healthcheck_url;
COMMIT;