BEGIN;
  ALTER TABLE workers
    ADD COLUMN healthcheck_url text;
COMMIT;