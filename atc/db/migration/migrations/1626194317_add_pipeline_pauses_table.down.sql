ALTER TABLE pipelines
  ADD COLUMN paused boolean DEFAULT FALSE;

UPDATE
  pipelines p
SET
  paused = TRUE
WHERE
  p.id IN (
    SELECT
      pp.pipeline_id
    FROM
      pipeline_pauses pp);

DROP TABLE pipeline_pauses;
