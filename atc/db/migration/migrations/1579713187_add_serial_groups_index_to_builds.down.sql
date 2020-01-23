BEGIN;
  DROP INDEX builds_running_builds_by_serial_groups_idx;
  DROP INDEX worker_resource_cert_volumes_idx;
  DROP INDEX pending_builds_idx;
  DROP INDEX next_build_pipes_job_id_idx;
COMMIT;
