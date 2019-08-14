BEGIN;
  ALTER TABLE builds
    DROP COLUMN "rerun_of",
    DROP COLUMN "rerun_build_number_seq";
COMMIT;
