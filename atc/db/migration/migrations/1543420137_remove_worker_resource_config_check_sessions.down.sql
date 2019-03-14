BEGIN;
  CREATE TABLE worker_resource_config_check_sessions (
      id integer NOT NULL,
      worker_base_resource_type_id integer,
      resource_config_check_session_id integer
  );

  CREATE SEQUENCE worker_resource_config_check_sessions_id_seq
      START WITH 1
      INCREMENT BY 1
      NO MINVALUE
      NO MAXVALUE
      CACHE 1;

  CREATE UNIQUE INDEX worker_resource_config_check_sessions_uniq
  ON worker_resource_config_check_sessions (resource_config_check_session_id, worker_base_resource_type_id);

  ALTER SEQUENCE worker_resource_config_check_sessions_id_seq OWNED BY worker_resource_config_check_sessions.id;

  ALTER TABLE ONLY worker_resource_config_check_sessions ALTER COLUMN id SET DEFAULT nextval('worker_resource_config_check_sessions_id_seq'::regclass);

  ALTER TABLE ONLY worker_resource_config_check_sessions
      ADD CONSTRAINT worker_resource_config_check_sessions_pkey PRIMARY KEY (id);

  ALTER TABLE ONLY worker_resource_config_check_sessions
      ADD CONSTRAINT worker_resource_config_check_resource_config_check_session_fkey FOREIGN KEY (resource_config_check_session_id) REFERENCES resource_config_check_sessions(id) ON DELETE CASCADE;

  ALTER TABLE ONLY worker_resource_config_check_sessions
      ADD CONSTRAINT worker_resource_config_check__worker_base_resource_type_id_fkey FOREIGN KEY (worker_base_resource_type_id) REFERENCES worker_base_resource_types(id) ON DELETE CASCADE;

  ALTER TABLE containers
      DROP COLUMN resource_config_check_session_id,
      ADD COLUMN worker_resource_config_check_session_id integer;

  ALTER TABLE ONLY containers
      ADD CONSTRAINT containers_worker_resource_config_check_session_id_fkey FOREIGN KEY (worker_resource_config_check_session_id) REFERENCES worker_resource_config_check_sessions(id) ON DELETE SET NULL;

  CREATE INDEX containers_worker_resource_config_check_session_id ON containers USING btree (worker_resource_config_check_session_id);

  ALTER TABLE resource_config_check_sessions
      DROP COLUMN worker_base_resource_type_id;
COMMIT;
