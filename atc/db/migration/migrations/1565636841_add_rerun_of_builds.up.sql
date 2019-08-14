BEGIN;
  ALTER TABLE builds
    ADD COLUMN "rerun_of" integer REFERENCES builds (id) ON DELETE CASCADE,
    ADD COLUMN "rerun_build_number_seq" integer DEFAULT 0 NOT NULL;
COMMIT;
