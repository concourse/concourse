ALTER TABLE jobs
  ADD COLUMN paused boolean DEFAULT FALSE;

UPDATE
  jobs j
SET
  paused = TRUE
WHERE
  j.id IN (
    SELECT
      jp.job_id
    FROM
      job_pauses jp);

DROP TABLE job_pauses;
