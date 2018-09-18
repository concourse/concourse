BEGIN;
  ALTER TABLE "workers" ADD COLUMN "certs_path" text;
  CREATE TABLE "worker_resource_certs" (
      "id" serial,
      "worker_name" text,
      "certs_path" text,
      PRIMARY KEY ("id"),
      CONSTRAINT "worker_resource_certs_worker_name_fkey" FOREIGN KEY ("worker_name") REFERENCES "workers"("name") ON DELETE CASCADE ON UPDATE SET NULL
  );
  ALTER TABLE "volumes"
  ADD COLUMN "worker_resource_certs_id" integer,
  ADD CONSTRAINT "worker_resource_certs_id_fkey" FOREIGN KEY ("worker_resource_certs_id") REFERENCES "worker_resource_certs"("id") ON DELETE SET NULL,
  DROP CONSTRAINT "cannot_invalidate_during_initialization",
  ADD CONSTRAINT "cannot_invalidate_during_initialization" CHECK ((state = ANY (ARRAY['created'::volume_state, 'destroying'::volume_state, 'failed'::volume_state])) AND worker_resource_cache_id IS NULL AND worker_base_resource_type_id IS NULL AND worker_task_cache_id IS NULL AND container_id IS NULL OR worker_resource_cache_id IS NOT NULL OR worker_base_resource_type_id IS NOT NULL OR worker_task_cache_id IS NOT NULL OR container_id IS NOT NULL OR worker_resource_certs_id IS NOT NULL);
COMMIT;

