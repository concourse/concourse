BEGIN;
  DROP INDEX ready_and_scheduled_ordering_with_rerun_builds_idx;

  DROP INDEX succeeded_builds_ordering_with_rerun_builds_idx;

  DROP INDEX successful_build_outputs_ordering_with_rerun_builds_idx;

  DROP INDEX rerun_of_builds_idx;
COMMIT;
