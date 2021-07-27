ALTER TABLE pipelines
  ADD COLUMN paused_by text,
  ADD COLUMN paused_at timestamptz;
