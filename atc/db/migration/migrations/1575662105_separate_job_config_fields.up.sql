BEGIN;
  ALTER TABLE jobs
    ADD COLUMN public boolean NOT NULL DEFAULT FALSE,
    ADD COLUMN max_in_flight int;
COMMIT;
