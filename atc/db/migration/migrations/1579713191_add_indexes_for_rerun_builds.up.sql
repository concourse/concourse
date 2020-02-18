BEGIN;
  CREATE INDEX ready_and_scheduled_ordering_with_rerun_builds_idx ON builds (job_id, COALESCE(rerun_of, id) DESC, id DESC) WHERE inputs_ready AND scheduled;

  CREATE INDEX succeeded_builds_ordering_with_rerun_builds_idx ON builds (job_id, rerun_of, COALESCE(rerun_of, id) DESC, id DESC) WHERE status = 'succeeded';

  CREATE INDEX successful_build_outputs_ordering_with_rerun_builds_idx ON successful_build_outputs (job_id, rerun_of, COALESCE(rerun_of, build_id) DESC, build_id DESC);

  CREATE INDEX rerun_of_builds_idx ON builds (rerun_of);
COMMIT;
