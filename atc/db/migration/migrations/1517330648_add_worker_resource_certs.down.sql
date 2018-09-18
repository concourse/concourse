BEGIN;
  ALTER TABLE "workers" DROP COLUMN "certs_path";
  ALTER TABLE "volumes"
  DROP CONSTRAINT "cannot_invalidate_during_initialization",
  ADD CONSTRAINT "cannot_invalidate_during_initialization" CHECK ((state = ANY (ARRAY['created'::volume_state, 'destroying'::volume_state, 'failed'::volume_state])) AND worker_resource_cache_id IS NULL AND worker_base_resource_type_id IS NULL AND worker_task_cache_id IS NULL AND container_id IS NULL OR worker_resource_cache_id IS NOT NULL OR worker_base_resource_type_id IS NOT NULL OR worker_task_cache_id IS NOT NULL OR container_id IS NOT NULL),
  DROP COLUMN "worker_resource_certs_id";
  DROP TABLE "worker_resource_certs";
COMMIT;
