BEGIN;

  ALTER TABLE builds
    ADD COLUMN create_time timestamp with time zone NOT NULL DEFAULT NOW();

COMMIT;
