BEGIN;
  ALTER TABLE build_image_resource_caches
    ADD COLUMN job_id integer REFERENCES jobs(id);

  UPDATE build_image_resource_caches
  SET job_id = builds.job_id
  FROM builds
  WHERE builds.id = build_id;

  CREATE INDEX build_image_resource_caches_job_build_idx ON build_image_resource_caches(job_id, build_id);
COMMIT;
