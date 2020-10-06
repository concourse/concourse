BEGIN;
  CREATE INDEX successful_build_outputs_rerun_of_idx ON successful_build_outputs (rerun_of);
COMMIT;
