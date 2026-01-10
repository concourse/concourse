
ALTER TABLE builds RENAME COLUMN "rerun_of" TO "rerun_of_old";

ALTER TABLE builds ADD COLUMN "rerun_of" bigint REFERENCES builds (id) ON DELETE CASCADE;

ALTER INDEX rerun_of_builds_idx RENAME TO rerun_of_old_builds_idx;
CREATE INDEX rerun_of_builds_idx ON builds (rerun_of);

DROP INDEX succeeded_builds_ordering_with_rerun_builds_idx;
CREATE INDEX succeeded_builds_ordering_with_rerun_builds_idx ON builds (job_id)
    INCLUDE (id, rerun_of, rerun_of_old)
    WHERE status = 'succeeded';

DROP INDEX order_builds_by_rerun_of_or_id_idx;
CREATE INDEX order_builds_by_rerun_of_or_id_idx ON builds(
    COALESCE(rerun_of, rerun_of_old, id) DESC,
    id DESC);

DROP INDEX order_job_builds_by_rerun_of_or_id_idx;
CREATE INDEX order_job_builds_by_rerun_of_or_id_idx ON builds (
    job_id,
    COALESCE(rerun_of, rerun_of_old, id) DESC,
    id DESC
) WHERE job_id IS NOT NULL;

-- Table stats are outdated after creating the above indexes. After testing,
-- found that we needed to force postgres to update stats on builds, otherwise
-- query plans were severly inefficient when querying the new run_of column.
ANALYZE builds;
