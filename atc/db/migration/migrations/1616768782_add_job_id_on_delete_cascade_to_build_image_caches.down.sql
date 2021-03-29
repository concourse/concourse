  ALTER TABLE build_image_resource_caches
    DROP CONSTRAINT build_image_resource_caches_job_id_fkey,
    ADD CONSTRAINT build_image_resource_caches_job_id_fkey FOREIGN KEY (job_id) REFERENCES jobs(id);
COMMIT;
