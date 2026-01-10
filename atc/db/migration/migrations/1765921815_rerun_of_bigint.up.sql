
ALTER TABLE builds RENAME COLUMN "rerun_of" TO "rerun_of_old";

ALTER TABLE builds ADD COLUMN "rerun_of" bigint REFERENCES builds (id) ON DELETE CASCADE;

ALTER INDEX rerun_of_builds_idx RENAME TO rerun_of_old_builds_idx;
CREATE INDEX rerun_of_builds_idx ON builds (rerun_of);

ALTER INDEX succeeded_builds_ordering_with_rerun_builds_idx RENAME TO succeeded_builds_ordering_with_rerun_old_builds_idx;
CREATE INDEX succeeded_builds_ordering_with_rerun_builds_idx ON builds (job_id, rerun_of, COALESCE(rerun_of, id) DESC, id DESC) WHERE status = 'succeeded';

ALTER INDEX order_builds_by_rerun_of_or_id_idx RENAME TO order_builds_by_rerun_of_old_or_id_idx;
CREATE INDEX order_builds_by_rerun_of_or_id_idx ON builds((COALESCE(rerun_of, id)) DESC, id DESC);

ALTER INDEX order_job_builds_by_rerun_of_or_id_idx RENAME TO order_job_builds_by_rerun_of_old_or_id_idx;
CREATE INDEX order_job_builds_by_rerun_of_or_id_idx ON builds (
    job_id,
    COALESCE(rerun_of, rerun_of_old, id) DESC,
    id DESC
)
WHERE job_id IS NOT NULL;

-- Table stats are outdated after creating the above indexes. After testing,
-- found that we needed to force postgres to update stats on builds, otherwise
-- query plans were severly inefficient when querying the new run_of column.
ANALYZE builds;
