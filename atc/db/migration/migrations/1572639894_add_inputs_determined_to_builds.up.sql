BEGIN;
  ALTER TABLE builds
    ADD COLUMN inputs_ready boolean NOT NULL DEFAULT false;

  UPDATE builds
  SET inputs_ready = scheduled;
COMMIT;
