BEGIN;
  ALTER TABLE volumes
  ADD COLUMN resource_version text,
  ADD COLUMN resource_hash text,
  ADD COLUMN original_volume_handle text,
  ADD COLUMN output_name text,
  ADD COLUMN host_path_version text,
  ADD COLUMN replicated_from text;
COMMIT;
