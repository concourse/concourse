ALTER TABLE pipelines
  ADD COLUMN paused_by text DEFAULT NULL,
  ADD COLUMN paused_at timestamptz DEFAULT NULL;
