CREATE INDEX order_job_builds_by_rerun_of_or_id_idx
  ON builds (job_id, COALESCE(rerun_of, id) DESC, id DESC)
  WHERE job_id IS NOT NULL;
