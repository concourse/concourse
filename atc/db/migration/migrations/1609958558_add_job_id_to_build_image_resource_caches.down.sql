BEGIN;
  ALTER TABLE build_image_resource_caches
    DROP COLUMN IF EXISTS job_id;

  CREATE INDEX build_image_resource_caches_build_id ON build_image_resource_caches USING btree (build_id);
COMMIT;
