BEGIN;
  DROP INDEX order_builds_by_rerun_of_or_id_idx;
  CREATE INDEX ready_and_scheduled_ordering_with_rerun_builds_idx ON builds (job_id, COALESCE(rerun_of, id) DESC, id DESC) WHERE inputs_ready AND scheduled;
COMMIT;
