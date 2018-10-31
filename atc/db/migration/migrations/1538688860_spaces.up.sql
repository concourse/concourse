BEGIN;

  CREATE TABLE spaces (
    id serial PRIMARY KEY,
    resource_config_id int REFERENCES resource_configs (id) ON DELETE CASCADE,
    name text NOT NULL,
    UNIQUE (resource_config_id, name)
  );

  CREATE TABLE resource_versions (
      "id" serial NOT NULL PRIMARY KEY,
      "space_id" integer NOT NULL REFERENCES spaces (id) ON DELETE CASCADE,
      "version" jsonb NOT NULL,
      "version_md5" text NOT NULL,
      "metadata" jsonb NOT NULL DEFAULT 'null',
      "check_order" integer NOT NULL DEFAULT 0
  );

  ALTER TABLE resource_versions
    ADD CONSTRAINT space_id_and_version_md5_unique UNIQUE (space_id, version_md5);

  ALTER TABLE resource_configs
    ADD COLUMN "default_space" text;

  CREATE UNIQUE INDEX job_combinations_job_id_combination_key ON job_combinations (job_id, combination);
  CREATE INDEX job_combinations_latest_completed_build_id ON job_combinations (latest_completed_build_id);
  CREATE INDEX job_combinations_next_build_id ON job_combinations (next_build_id);
  CREATE INDEX job_combinations_transition_build_id ON job_combinations (transition_build_id);

  CREATE TABLE job_combination_spaces (
    job_combination_id int REFERENCES job_combinations (id) ON DELETE CASCADE,
    space_id int REFERENCES spaces (id) ON DELETE CASCADE
  );

  CREATE UNIQUE INDEX job_combination_spaces_job_combination_id_space_id_key ON job_combination_spaces (job_combination_id, space_id);

  -- ALTER TABLE builds RENAME job_id TO job_combination_id;
  -- ALTER TABLE builds DROP CONSTRAINT fkey_job_id;
  -- ALTER TABLE builds ADD CONSTRAINT fkey_job_combination_id FOREIGN KEY (job_combination_id) REFERENCES job_combinations(id) ON DELETE CASCADE;
  -- ALTER INDEX builds_job_id RENAME TO builds_job_combination_id;

  UPDATE job_combinations c SET latest_completed_build_id = j.latest_completed_build_id FROM jobs j WHERE c.job_id = j.id;
  UPDATE job_combinations c SET next_build_id = j.next_build_id FROM jobs j WHERE c.job_id = j.id;
  UPDATE job_combinations c SET transition_build_id = j.transition_build_id FROM jobs j WHERE c.job_id = j.id;

  ALTER TABLE jobs
    DROP COLUMN build_number_seq,
    DROP COLUMN inputs_determined,
    DROP COLUMN latest_completed_build_id,
    DROP COLUMN next_build_id,
    DROP COLUMN transition_build_id;

  -- ALTER TABLE independent_build_inputs RENAME job_id TO job_combination_id;
  -- ALTER TABLE independent_build_inputs DROP CONSTRAINT independent_build_inputs_unique_job_id_input_name;
  -- ALTER TABLE independent_build_inputs ADD CONSTRAINT independent_build_inputs_unique_job_combination_id_input_name UNIQUE (job_combination_id, input_name);
  -- ALTER TABLE independent_build_inputs DROP CONSTRAINT independent_build_inputs_job_id_fkey;
  -- ALTER TABLE independent_build_inputs ADD CONSTRAINT independent_build_inputs_job_combination_id_fkey FOREIGN KEY (job_combination_id) REFERENCES job_combinations(id) ON DELETE CASCADE;
  -- ALTER INDEX independent_build_inputs_job_id RENAME TO independent_build_inputs_job_combination_id;

  -- ALTER TABLE next_build_inputs RENAME job_id TO job_combination_id;
  -- ALTER TABLE next_build_inputs DROP CONSTRAINT next_build_inputs_unique_job_id_input_name;
  -- ALTER TABLE next_build_inputs ADD CONSTRAINT next_build_inputs_unique_job_combination_id_input_name UNIQUE (job_combination_id, input_name);
  -- ALTER TABLE next_build_inputs DROP CONSTRAINT next_build_inputs_job_id_fkey;
  -- ALTER TABLE next_build_inputs ADD CONSTRAINT next_build_inputs_job_combination_id_fkey FOREIGN KEY (job_combination_id) REFERENCES job_combinations(id) ON DELETE CASCADE;
  -- ALTER INDEX next_build_inputs_job_id RENAME TO next_build_inputs_job_combination_id;

  -- ALTER TABLE worker_task_caches RENAME job_id TO job_combination_id;
  -- ALTER TABLE worker_task_caches DROP CONSTRAINT worker_task_caches_job_id_fkey;
  -- ALTER TABLE worker_task_caches ADD CONSTRAINT worker_task_caches_job_combination_id_fkey FOREIGN KEY (job_combination_id) REFERENCES job_combinations(id) ON DELETE CASCADE;
  -- ALTER INDEX worker_task_caches_job_id RENAME TO worker_task_caches_job_combination_id;

  ALTER TABLE resource_caches DROP COLUMN metadata;

 COMMIT;
