CREATE TABLE pipeline_pauses (
  pipeline_id integer PRIMARY KEY,
  paused boolean NOT NULL DEFAULT TRUE,
  paused_by text DEFAULT NULL,
  paused_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE pipeline_pauses
  ADD CONSTRAINT pipeline_pauses_id_fkey FOREIGN KEY (pipeline_id) REFERENCES pipelines (id) ON DELETE CASCADE;

INSERT INTO pipeline_pauses
SELECT
  id,
  TRUE
FROM
  pipelines
WHERE
  pipelines.paused = TRUE;

ALTER TABLE pipelines
  DROP COLUMN paused;
