BEGIN;

  ALTER TABLE worker_task_caches
    ADD job_id integer,
    ADD step_name text,
    ADD path text;

  UPDATE worker_task_caches
  SET job_id=tc.job_id,
      step_name=tc.step_name,
      path=tc.path
  FROM task_caches tc
  WHERE worker_task_caches.task_cache_id=tc.id;

  ALTER TABLE worker_task_caches
    ALTER COLUMN step_name SET NOT NULL,
    ALTER COLUMN path SET NOT NULL;

  DROP INDEX task_caches_job_id_step_name_path_uniq;

  DROP INDEX worker_task_caches_worker_name_task_cache_id_uniq;
  CREATE UNIQUE INDEX worker_task_caches_uniq
    ON worker_task_caches (job_id, step_name, worker_name, path);

  CREATE INDEX worker_task_caches_job_id ON worker_task_caches USING btree (job_id);

  ALTER TABLE ONLY worker_task_caches
    ADD CONSTRAINT worker_task_caches_job_id_fkey FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE;

  ALTER TABLE worker_task_caches DROP CONSTRAINT worker_task_caches_task_cache_fkey;
  ALTER TABLE ONLY worker_task_caches
    DROP COLUMN task_cache_id;

  DROP TABLE task_caches;

COMMIT;
