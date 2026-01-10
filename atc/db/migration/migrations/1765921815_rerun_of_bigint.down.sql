
DROP INDEX rerun_of_builds_idx;
ALTER INDEX rerun_of_old_builds_idx RENAME TO rerun_of_builds_idx;

DROP INDEX succeeded_builds_ordering_with_rerun_builds_idx;
CREATE INDEX succeeded_builds_ordering_with_rerun_builds_idx ON builds (job_id, rerun_of, COALESCE(rerun_of, id) DESC, id DESC) WHERE status = 'succeeded';

DROP INDEX order_builds_by_rerun_of_or_id_idx;
CREATE INDEX order_builds_by_rerun_of_or_id_idx ON builds((COALESCE(rerun_of, id)) DESC, id DESC);

DROP INDEX order_job_builds_by_rerun_of_or_id_idx;
CREATE INDEX order_job_builds_by_rerun_of_or_id_idx
  ON builds (job_id, COALESCE(rerun_of, id) DESC, id DESC)
  WHERE job_id IS NOT NULL;

ALTER TABLE builds DROP COLUMN "rerun_of";

ALTER TABLE builds RENAME COLUMN "rerun_of_old" TO "rerun_of";
