BEGIN;
  ALTER TABLE builds
    DROP COLUMN "rerun_of",
    DROP COLUMN "rerun_number";

  ALTER TABLE successful_build_outputs
    DROP COLUMN "rerun_of";
COMMIT;
