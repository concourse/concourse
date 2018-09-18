BEGIN;
  ALTER TABLE volumes
  DROP COLUMN resource_version,
  DROP COLUMN resource_hash,
  DROP COLUMN original_volume_handle,
  DROP COLUMN output_name,
  DROP COLUMN host_path_version,
  DROP COLUMN replicated_from;
COMMIT;
