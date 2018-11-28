BEGIN;
  ALTER TABLE containers
      DROP COLUMN worker_resource_config_check_session_id;

  DROP TABLE worker_resource_config_check_sessions;

  TRUNCATE TABLE resource_config_check_sessions;

  ALTER TABLE resource_config_check_sessions
      ADD COLUMN worker_base_resource_type_id integer,
      ADD CONSTRAINT resource_config_check_sessions_worker_base_resource_type_id_fkey FOREIGN KEY (worker_base_resource_type_id) REFERENCES worker_base_resource_types(id) ON DELETE CASCADE;

  CREATE UNIQUE INDEX resource_config_check_sessions_uniq
  ON resource_config_check_sessions (resource_config_id, worker_base_resource_type_id);

  ALTER TABLE containers
      ADD COLUMN resource_config_check_session_id integer,
      ADD CONSTRAINT containers_resource_config_check_session_id_fkey FOREIGN KEY (resource_config_check_session_id) REFERENCES resource_config_check_sessions(id) ON DELETE SET NULL;

  CREATE INDEX containers_resource_config_check_session_id ON containers USING btree (resource_config_check_session_id);
COMMIT;
