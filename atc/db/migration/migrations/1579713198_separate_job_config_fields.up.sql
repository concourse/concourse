BEGIN;
  ALTER TABLE jobs
    ADD COLUMN public boolean NOT NULL DEFAULT FALSE,
    ADD COLUMN max_in_flight int NOT NULL DEFAULT 0,
    ADD COLUMN disable_manual_trigger boolean NOT NULL DEFAULT FALSE;
COMMIT;
