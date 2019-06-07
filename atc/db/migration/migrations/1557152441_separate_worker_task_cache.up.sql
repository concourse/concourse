BEGIN;
  CREATE TABLE task_caches (
    id serial,
    worker_name text,
    job_id integer,
    step_name text NOT NULL,
    path text NOT NULL,
    PRIMARY KEY ("id")
  );
  CREATE UNIQUE INDEX task_caches_job_id_step_name_path_uniq
    ON task_caches (job_id, step_name, path);

  CREATE INDEX task_caches_job_id ON task_caches USING btree (job_id);

  ALTER TABLE ONLY task_caches
    ADD CONSTRAINT task_caches_job_id_fkey FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE;

  ALTER TABLE worker_task_caches
    ADD COLUMN task_cache_id INTEGER;

  ALTER TABLE worker_task_caches
    ADD CONSTRAINT worker_task_caches_task_cache_fkey FOREIGN KEY (task_cache_id) REFERENCES task_caches (id) ON DELETE CASCADE;
  CREATE INDEX worker_task_caches_task_cache_id ON worker_task_caches (task_cache_id);

  WITH ins AS (
    INSERT INTO task_caches (worker_name, job_id, step_name, path)
    SELECT DISTINCT wtc.worker_name, wtc.job_id, wtc.step_name, wtc.path
    FROM worker_task_caches wtc
    ON CONFLICT DO NOTHING
    RETURNING *
  )
  UPDATE worker_task_caches wtc
  SET task_cache_id = ins.id
  FROM ins
  WHERE ins.job_id = wtc.job_id
    AND ins.step_name = wtc.step_name
    AND ins.path = wtc.path;

  DROP INDEX worker_task_caches_uniq;
  CREATE UNIQUE INDEX worker_task_caches_worker_name_task_cache_id_uniq
    ON worker_task_caches (worker_name, task_cache_id);


  ALTER TABLE worker_task_caches
    DROP COLUMN job_id,
    DROP COLUMN step_name,
    DROP COLUMN path;

  ALTER TABLE task_caches
    DROP COLUMN worker_name;

COMMIT;
