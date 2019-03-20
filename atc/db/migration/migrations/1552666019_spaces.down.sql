BEGIN;
  ALTER TABLE resource_caches ADD COLUMN metadata text;

  ALTER TABLE worker_task_caches RENAME job_combination_id TO job_id;
  ALTER TABLE worker_task_caches DROP CONSTRAINT worker_task_caches_job_combination_id_fkey;
  ALTER TABLE worker_task_caches ADD CONSTRAINT worker_task_caches_job_id_fkey FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE;
  ALTER INDEX worker_task_caches_job_combination_id RENAME TO worker_task_caches_job_id;

  ALTER TABLE next_build_inputs RENAME job_combination_id TO job_id;
  ALTER TABLE next_build_inputs DROP CONSTRAINT next_build_inputs_unique_job_combination_id_input_name;
  ALTER TABLE next_build_inputs ADD CONSTRAINT next_build_inputs_unique_job_id_input_name UNIQUE (job_id, input_name);
  ALTER TABLE next_build_inputs DROP CONSTRAINT next_build_inputs_job_combination_id_fkey;
  ALTER TABLE next_build_inputs ADD CONSTRAINT next_build_inputs_job_id_fkey FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE;
  ALTER INDEX next_build_inputs_job_combination_id RENAME TO next_build_inputs_job_id;

  ALTER TABLE independent_build_inputs RENAME job_combination_id TO job_id;
  ALTER TABLE independent_build_inputs DROP CONSTRAINT independent_build_inputs_unique_job_combination_id_input_name;
  ALTER TABLE independent_build_inputs ADD CONSTRAINT independent_build_inputs_unique_job_id_input_name UNIQUE (job_id, input_name);
  ALTER TABLE independent_build_inputs DROP CONSTRAINT independent_build_inputs_job_combination_id_fkey;
  ALTER TABLE independent_build_inputs ADD CONSTRAINT independent_build_inputs_job_id_fkey FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE;
  ALTER INDEX independent_build_inputs_job_combination_id RENAME TO independent_build_inputs_job_id;

  DROP INDEX job_combinations_latest_completed_build_id;
  DROP INDEX job_combinations_next_build_id;
  DROP INDEX job_combinations_transition_build_id;

  ALTER TABLE jobs
    ADD COLUMN latest_completed_build_id integer REFERENCES builds (id) ON DELETE SET NULL,
    ADD COLUMN next_build_id integer REFERENCES builds (id) ON DELETE SET NULL,
    ADD COLUMN transition_build_id integer REFERENCES builds (id) ON DELETE SET NULL;

  CREATE INDEX jobs_latest_completed_build_id ON jobs (latest_completed_build_id);
  CREATE INDEX jobs_next_build_id ON jobs (next_build_id);
  CREATE INDEX jobs_transition_build_id ON jobs (transition_build_id);

  ALTER TABLE builds RENAME job_combination_id TO job_id;
  ALTER TABLE builds DROP CONSTRAINT fkey_job_combination_id;
  ALTER TABLE builds ADD CONSTRAINT fkey_job_id FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE;
  ALTER INDEX builds_job_combination_id RENAME TO builds_job_id;

  ALTER TABLE jobs ADD COLUMN inputs_determined boolean DEFAULT false NOT NULL;
  ALTER TABLE jobs ADD COLUMN build_number_seq integer DEFAULT 0 NOT NULL;
   UPDATE jobs SET build_number_seq = (
    SELECT COUNT(*) FROM builds b
    LEFT JOIN job_combinations c ON b.job_combination_id = c.id
    LEFT JOIN jobs j ON c.job_id = j.id
    WHERE j.id = jobs.id
  );

  DROP INDEX job_combination_spaces_job_combination_id_space_id_key;
  DROP TABLE job_combination_spaces;

  DROP INDEX job_combinations_job_id_combination_key;

  ALTER TABLE resource_configs
    DROP COLUMN "default_space";

  DROP TABLE resource_versions;

  DROP TABLE spaces;

COMMIT;
