BEGIN;
  ALTER TABLE builds
    ADD COLUMN "retrigger_of" integer REFERENCES builds (id) ON DELETE CASCADE,
    ADD COLUMN "retrigger_build_num_seq" integer DEFAULT 0 NOT NULL;

COMMIT;
