BEGIN;
  ALTER TABLE builds
    DROP COLUMN "retrigger_of",
    DROP COLUMN "retrigger_build_num_seq";
COMMIT;
