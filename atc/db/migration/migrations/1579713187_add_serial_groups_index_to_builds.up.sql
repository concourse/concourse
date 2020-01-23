BEGIN;
  CREATE INDEX builds_running_builds_by_serial_groups_idx ON builds (job_id) WHERE completed = false AND scheduled = true;

  CREATE INDEX worker_resource_cert_volumes_idx ON volumes (worker_name, worker_resource_certs_id);

  CREATE INDEX pending_builds_idx ON builds (job_id, id ASC) WHERE status = 'pending';

  CREATE INDEX next_build_pipes_job_id_idx ON next_build_pipes (to_job_id);
COMMIT;
