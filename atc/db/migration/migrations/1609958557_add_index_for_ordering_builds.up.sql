
DROP INDEX IF EXISTS ready_and_scheduled_ordering_with_rerun_builds_idx;
CREATE INDEX order_builds_by_rerun_of_or_id_idx ON builds((COALESCE(rerun_of, id)) DESC, id DESC);

