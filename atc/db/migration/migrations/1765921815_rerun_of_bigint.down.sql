
DROP INDEX rerun_of_builds_idx;
ALTER INDEX rerun_of_old_builds_idx RENAME TO rerun_of_builds_idx;

DROP INDEX succeeded_builds_ordering_with_rerun_builds_idx;
ALTER INDEX succeeded_builds_ordering_with_rerun_old_builds_idx RENAME TO succeeded_builds_ordering_with_rerun_builds_idx;

DROP INDEX order_builds_by_rerun_of_or_id_idx;
ALTER INDEX order_builds_by_rerun_of_old_or_id_idx RENAME TO order_builds_by_rerun_of_or_id_idx;

DROP INDEX order_job_builds_by_rerun_of_or_id_idx;
ALTER INDEX order_job_builds_by_rerun_of_old_or_id_idx RENAME TO order_job_builds_by_rerun_of_or_id_idx;

ALTER TABLE builds DROP COLUMN "rerun_of";

ALTER TABLE builds RENAME COLUMN "rerun_of_old" TO "rerun_of";
