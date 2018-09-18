BEGIN;

  CREATE TYPE build_status AS ENUM (
        'pending',
      'started',
      'aborted',
      'succeeded',
      'failed',
      'errored'
  );



  CREATE TYPE container_stage AS ENUM (
      'check',
      'get',
      'run'
  );



  CREATE TYPE container_state AS ENUM (
      'creating',
      'created',
      'destroying',
      'failed'
  );



  CREATE TYPE container_state_old AS ENUM (
      'creating',
      'created',
      'destroying'
  );



  CREATE TYPE volume_state AS ENUM (
      'creating',
      'created',
      'destroying',
      'failed'
  );



  CREATE TYPE volume_state_old AS ENUM (
      'creating',
      'created',
      'destroying'
  );



  CREATE TYPE worker_state AS ENUM (
      'running',
      'stalled',
      'landing',
      'landed',
      'retiring'
  );


  CREATE TABLE base_resource_types (
      id integer NOT NULL,
      name text NOT NULL
  );



  CREATE SEQUENCE base_resource_types_id_seq
      START WITH 1
      INCREMENT BY 1
      NO MINVALUE
      NO MAXVALUE
      CACHE 1;



  ALTER SEQUENCE base_resource_types_id_seq OWNED BY base_resource_types.id;



  CREATE TABLE build_events (
      build_id integer,
      type character varying(32) NOT NULL,
      payload text NOT NULL,
      event_id integer NOT NULL,
      version text NOT NULL
  );



  CREATE TABLE build_image_resource_caches (
      resource_cache_id integer,
      build_id integer NOT NULL
  );



  CREATE TABLE build_inputs (
      build_id integer,
      versioned_resource_id integer,
      name text NOT NULL,
      modified_time timestamp without time zone DEFAULT now() NOT NULL
  );



  CREATE TABLE build_outputs (
      build_id integer,
      versioned_resource_id integer,
      explicit boolean DEFAULT false NOT NULL,
      modified_time timestamp without time zone DEFAULT now() NOT NULL
  );



  CREATE TABLE builds (
      id integer NOT NULL,
      name text NOT NULL,
      status build_status NOT NULL,
      scheduled boolean DEFAULT false NOT NULL,
      start_time timestamp with time zone,
      end_time timestamp with time zone,
      engine character varying(16),
      engine_metadata text,
      completed boolean DEFAULT false NOT NULL,
      job_id integer,
      reap_time timestamp with time zone,
      team_id integer NOT NULL,
      manually_triggered boolean DEFAULT false,
      interceptible boolean DEFAULT true,
      nonce text,
      public_plan json DEFAULT '{}'::json,
      pipeline_id integer
  );



  CREATE SEQUENCE builds_id_seq
      START WITH 1
      INCREMENT BY 1
      NO MINVALUE
      NO MAXVALUE
      CACHE 1;



  ALTER SEQUENCE builds_id_seq OWNED BY builds.id;



  CREATE TABLE cache_invalidator (
      last_invalidated timestamp without time zone DEFAULT '1970-01-01 00:00:00'::timestamp without time zone NOT NULL
  );



  CREATE SEQUENCE config_version_seq
      START WITH 1
      INCREMENT BY 1
      NO MINVALUE
      NO MAXVALUE
      CACHE 1;



  CREATE TABLE containers (
      handle text NOT NULL,
      build_id integer,
      plan_id text,
      pipeline_id integer,
      resource_id integer,
      worker_name text,
      best_if_used_by timestamp without time zone,
      id integer NOT NULL,
      team_id integer,
      state container_state DEFAULT 'creating'::container_state NOT NULL,
      hijacked boolean DEFAULT false NOT NULL,
      discontinued boolean DEFAULT false NOT NULL,
      meta_type text DEFAULT ''::text NOT NULL,
      meta_step_name text DEFAULT ''::text NOT NULL,
      meta_attempt text DEFAULT ''::text NOT NULL,
      meta_working_directory text DEFAULT ''::text NOT NULL,
      meta_process_user text DEFAULT ''::text NOT NULL,
      meta_pipeline_id integer DEFAULT 0 NOT NULL,
      meta_job_id integer DEFAULT 0 NOT NULL,
      meta_build_id integer DEFAULT 0 NOT NULL,
      meta_pipeline_name text DEFAULT ''::text NOT NULL,
      meta_job_name text DEFAULT ''::text NOT NULL,
      meta_build_name text DEFAULT ''::text NOT NULL,
      image_check_container_id integer,
      image_get_container_id integer,
      worker_resource_config_check_session_id integer
  );



  CREATE SEQUENCE containers_id_seq
      START WITH 1
      INCREMENT BY 1
      NO MINVALUE
      NO MAXVALUE
      CACHE 1;



  ALTER SEQUENCE containers_id_seq OWNED BY containers.id;



  CREATE TABLE independent_build_inputs (
      id integer NOT NULL,
      job_id integer NOT NULL,
      input_name text NOT NULL,
      version_id integer NOT NULL,
      first_occurrence boolean NOT NULL
  );



  CREATE SEQUENCE independent_build_inputs_id_seq
      START WITH 1
      INCREMENT BY 1
      NO MINVALUE
      NO MAXVALUE
      CACHE 1;



  ALTER SEQUENCE independent_build_inputs_id_seq OWNED BY independent_build_inputs.id;



  CREATE TABLE jobs (
      name text NOT NULL,
      build_number_seq integer DEFAULT 0 NOT NULL,
      paused boolean DEFAULT false,
      id integer NOT NULL,
      pipeline_id integer NOT NULL,
      first_logged_build_id integer DEFAULT 0 NOT NULL,
      inputs_determined boolean DEFAULT false NOT NULL,
      max_in_flight_reached boolean DEFAULT false NOT NULL,
      config text NOT NULL,
      active boolean DEFAULT false NOT NULL,
      interruptible boolean DEFAULT false NOT NULL,
      nonce text
  );



  CREATE SEQUENCE jobs_id_seq
      START WITH 1
      INCREMENT BY 1
      NO MINVALUE
      NO MAXVALUE
      CACHE 1;



  ALTER SEQUENCE jobs_id_seq OWNED BY jobs.id;



  CREATE TABLE jobs_serial_groups (
      id integer NOT NULL,
      serial_group text NOT NULL,
      job_id integer
  );



  CREATE SEQUENCE jobs_serial_groups_id_seq
      START WITH 1
      INCREMENT BY 1
      NO MINVALUE
      NO MAXVALUE
      CACHE 1;



  ALTER SEQUENCE jobs_serial_groups_id_seq OWNED BY jobs_serial_groups.id;



  CREATE MATERIALIZED VIEW latest_completed_builds_per_job AS
   WITH latest_build_ids_per_job AS (
           SELECT max(b_1.id) AS build_id
             FROM (builds b_1
               JOIN jobs j ON ((j.id = b_1.job_id)))
            WHERE (b_1.status <> ALL (ARRAY['pending'::build_status, 'started'::build_status]))
            GROUP BY b_1.job_id
          )
   SELECT b.id,
      b.name,
      b.status,
      b.scheduled,
      b.start_time,
      b.end_time,
      b.engine,
      b.engine_metadata,
      b.completed,
      b.job_id,
      b.reap_time,
      b.team_id,
      b.manually_triggered,
      b.interceptible,
      b.nonce,
      b.public_plan,
      b.pipeline_id
     FROM (builds b
       JOIN latest_build_ids_per_job l ON ((l.build_id = b.id)))
    WITH NO DATA;

  REFRESH MATERIALIZED VIEW latest_completed_builds_per_job;


  CREATE TABLE next_build_inputs (
      id integer NOT NULL,
      job_id integer NOT NULL,
      input_name text NOT NULL,
      version_id integer NOT NULL,
      first_occurrence boolean NOT NULL
  );



  CREATE SEQUENCE next_build_inputs_id_seq
      START WITH 1
      INCREMENT BY 1
      NO MINVALUE
      NO MAXVALUE
      CACHE 1;



  ALTER SEQUENCE next_build_inputs_id_seq OWNED BY next_build_inputs.id;



  CREATE MATERIALIZED VIEW next_builds_per_job AS
   WITH latest_build_ids_per_job AS (
           SELECT min(b_1.id) AS build_id
             FROM (builds b_1
               JOIN jobs j ON ((j.id = b_1.job_id)))
            WHERE (b_1.status = ANY (ARRAY['pending'::build_status, 'started'::build_status]))
            GROUP BY b_1.job_id
          )
   SELECT b.id,
      b.name,
      b.status,
      b.scheduled,
      b.start_time,
      b.end_time,
      b.engine,
      b.engine_metadata,
      b.completed,
      b.job_id,
      b.reap_time,
      b.team_id,
      b.manually_triggered,
      b.interceptible,
      b.nonce,
      b.public_plan,
      b.pipeline_id
     FROM (builds b
       JOIN latest_build_ids_per_job l ON ((l.build_id = b.id)))
    WITH NO DATA;

  REFRESH MATERIALIZED VIEW next_builds_per_job;


  CREATE SEQUENCE one_off_name
      START WITH 1
      INCREMENT BY 1
      NO MINVALUE
      NO MAXVALUE
      CACHE 1;



  CREATE TABLE pipelines (
      version integer DEFAULT 0 NOT NULL,
      id integer NOT NULL,
      name text NOT NULL,
      paused boolean DEFAULT false,
      ordering integer DEFAULT 0 NOT NULL,
      last_scheduled timestamp without time zone DEFAULT '1970-01-01 00:00:00'::timestamp without time zone NOT NULL,
      team_id integer NOT NULL,
      public boolean DEFAULT false NOT NULL,
      groups json
  );



  CREATE SEQUENCE pipelines_id_seq
      START WITH 1
      INCREMENT BY 1
      NO MINVALUE
      NO MAXVALUE
      CACHE 1;



  ALTER SEQUENCE pipelines_id_seq OWNED BY pipelines.id;



  CREATE TABLE pipes (
      id text NOT NULL,
      url text,
      team_id integer NOT NULL
  );



  CREATE TABLE resource_cache_uses (
      resource_cache_id integer,
      build_id integer,
      container_id integer
  );



  CREATE TABLE resource_caches (
      id integer NOT NULL,
      resource_config_id integer,
      version text NOT NULL,
      params_hash text NOT NULL,
      metadata text
  );



  CREATE SEQUENCE resource_caches_id_seq
      START WITH 1
      INCREMENT BY 1
      NO MINVALUE
      NO MAXVALUE
      CACHE 1;



  ALTER SEQUENCE resource_caches_id_seq OWNED BY resource_caches.id;



  CREATE TABLE resource_config_check_sessions (
      id integer NOT NULL,
      resource_config_id integer,
      expires_at timestamp with time zone
  );



  CREATE SEQUENCE resource_config_check_sessions_id_seq
      START WITH 1
      INCREMENT BY 1
      NO MINVALUE
      NO MAXVALUE
      CACHE 1;



  ALTER SEQUENCE resource_config_check_sessions_id_seq OWNED BY resource_config_check_sessions.id;



  CREATE TABLE resource_configs (
      id integer NOT NULL,
      base_resource_type_id integer,
      source_hash text NOT NULL,
      resource_cache_id integer
  );



  CREATE SEQUENCE resource_configs_id_seq
      START WITH 1
      INCREMENT BY 1
      NO MINVALUE
      NO MAXVALUE
      CACHE 1;



  ALTER SEQUENCE resource_configs_id_seq OWNED BY resource_configs.id;



  CREATE TABLE resource_types (
      id integer NOT NULL,
      pipeline_id integer,
      name text NOT NULL,
      type text NOT NULL,
      version text,
      last_checked timestamp without time zone DEFAULT '1970-01-01 00:00:00'::timestamp without time zone NOT NULL,
      config text NOT NULL,
      active boolean DEFAULT false NOT NULL,
      nonce text,
      resource_config_id integer
  );



  CREATE SEQUENCE resource_types_id_seq
      START WITH 1
      INCREMENT BY 1
      NO MINVALUE
      NO MAXVALUE
      CACHE 1;



  ALTER SEQUENCE resource_types_id_seq OWNED BY resource_types.id;



  CREATE TABLE resources (
      name text NOT NULL,
      check_error text,
      paused boolean DEFAULT false,
      id integer NOT NULL,
      pipeline_id integer NOT NULL,
      last_checked timestamp without time zone DEFAULT '1970-01-01 00:00:00'::timestamp without time zone NOT NULL,
      config text NOT NULL,
      active boolean DEFAULT false NOT NULL,
      nonce text,
      resource_config_id integer
  );



  CREATE SEQUENCE resources_id_seq
      START WITH 1
      INCREMENT BY 1
      NO MINVALUE
      NO MAXVALUE
      CACHE 1;



  ALTER SEQUENCE resources_id_seq OWNED BY resources.id;



  CREATE TABLE teams (
      id integer NOT NULL,
      name text NOT NULL,
      basic_auth json,
      admin boolean DEFAULT false,
      auth text,
      nonce text,
      CONSTRAINT constraint_teams_name_not_empty CHECK ((length(name) > 0))
  );



  CREATE SEQUENCE teams_id_seq
      START WITH 1
      INCREMENT BY 1
      NO MINVALUE
      NO MAXVALUE
      CACHE 1;



  ALTER SEQUENCE teams_id_seq OWNED BY teams.id;



  CREATE MATERIALIZED VIEW transition_builds_per_job AS
   WITH builds_before_transition AS (
           SELECT b_1.job_id,
              max(b_1.id) AS max
             FROM ((builds b_1
               LEFT JOIN jobs j ON ((b_1.job_id = j.id)))
               LEFT JOIN latest_completed_builds_per_job s ON ((b_1.job_id = s.job_id)))
            WHERE ((b_1.status <> s.status) AND (b_1.status <> ALL (ARRAY['pending'::build_status, 'started'::build_status])))
            GROUP BY b_1.job_id
          )
   SELECT DISTINCT ON (b.job_id) b.id,
      b.name,
      b.status,
      b.scheduled,
      b.start_time,
      b.end_time,
      b.engine,
      b.engine_metadata,
      b.completed,
      b.job_id,
      b.reap_time,
      b.team_id,
      b.manually_triggered,
      b.interceptible,
      b.nonce,
      b.public_plan,
      b.pipeline_id
     FROM (builds b
       LEFT JOIN builds_before_transition ON ((b.job_id = builds_before_transition.job_id)))
    WHERE (((builds_before_transition.max IS NULL) AND (b.status <> ALL (ARRAY['pending'::build_status, 'started'::build_status]))) OR (b.id > builds_before_transition.max))
    ORDER BY b.job_id, b.id
    WITH NO DATA;


  REFRESH MATERIALIZED VIEW transition_builds_per_job;


  CREATE TABLE versioned_resources (
      id integer NOT NULL,
      version text NOT NULL,
      metadata text NOT NULL,
      type text NOT NULL,
      enabled boolean DEFAULT true NOT NULL,
      resource_id integer,
      modified_time timestamp without time zone DEFAULT now() NOT NULL,
      check_order integer DEFAULT 0 NOT NULL
  );



  CREATE SEQUENCE versioned_resources_id_seq
      START WITH 1
      INCREMENT BY 1
      NO MINVALUE
      NO MAXVALUE
      CACHE 1;



  ALTER SEQUENCE versioned_resources_id_seq OWNED BY versioned_resources.id;



  CREATE TABLE volumes (
      id integer NOT NULL,
      handle text NOT NULL,
      resource_version text,
      resource_hash text,
      worker_name text NOT NULL,
      original_volume_handle text,
      output_name text,
      path text,
      host_path_version text,
      replicated_from text,
      container_id integer,
      team_id integer,
      state volume_state DEFAULT 'creating'::volume_state NOT NULL,
      parent_id integer,
      parent_state volume_state,
      worker_base_resource_type_id integer,
      worker_resource_cache_id integer,
      worker_task_cache_id integer,
      CONSTRAINT cannot_invalidate_during_initialization CHECK ((((state = ANY (ARRAY['created'::volume_state, 'destroying'::volume_state, 'failed'::volume_state])) AND ((worker_resource_cache_id IS NULL) AND (worker_base_resource_type_id IS NULL) AND (worker_task_cache_id IS NULL) AND (container_id IS NULL))) OR ((worker_resource_cache_id IS NOT NULL) OR (worker_base_resource_type_id IS NOT NULL) OR (worker_task_cache_id IS NOT NULL) OR (container_id IS NOT NULL))))
  );



  CREATE SEQUENCE volumes_id_seq
      START WITH 1
      INCREMENT BY 1
      NO MINVALUE
      NO MAXVALUE
      CACHE 1;



  ALTER SEQUENCE volumes_id_seq OWNED BY volumes.id;



  CREATE TABLE worker_base_resource_types (
      worker_name text,
      base_resource_type_id integer,
      image text NOT NULL,
      version text NOT NULL,
      id integer NOT NULL
  );



  CREATE SEQUENCE worker_base_resource_types_id_seq
      START WITH 1
      INCREMENT BY 1
      NO MINVALUE
      NO MAXVALUE
      CACHE 1;



  ALTER SEQUENCE worker_base_resource_types_id_seq OWNED BY worker_base_resource_types.id;



  CREATE TABLE worker_resource_caches (
      id integer NOT NULL,
      worker_base_resource_type_id integer,
      resource_cache_id integer
  );



  CREATE SEQUENCE worker_resource_caches_id_seq
      START WITH 1
      INCREMENT BY 1
      NO MINVALUE
      NO MAXVALUE
      CACHE 1;



  ALTER SEQUENCE worker_resource_caches_id_seq OWNED BY worker_resource_caches.id;



  CREATE TABLE worker_resource_config_check_sessions (
      id integer NOT NULL,
      worker_base_resource_type_id integer,
      resource_config_check_session_id integer,
      team_id integer
  );



  CREATE SEQUENCE worker_resource_config_check_sessions_id_seq
      START WITH 1
      INCREMENT BY 1
      NO MINVALUE
      NO MAXVALUE
      CACHE 1;



  ALTER SEQUENCE worker_resource_config_check_sessions_id_seq OWNED BY worker_resource_config_check_sessions.id;



  CREATE TABLE worker_task_caches (
      id integer NOT NULL,
      worker_name text,
      job_id integer,
      step_name text NOT NULL,
      path text NOT NULL
  );



  CREATE SEQUENCE worker_task_caches_id_seq
      START WITH 1
      INCREMENT BY 1
      NO MINVALUE
      NO MAXVALUE
      CACHE 1;



  ALTER SEQUENCE worker_task_caches_id_seq OWNED BY worker_task_caches.id;



  CREATE TABLE workers (
      addr text,
      expires timestamp with time zone,
      active_containers integer DEFAULT 0,
      resource_types text,
      platform text,
      tags text,
      baggageclaim_url text DEFAULT ''::text,
      name text NOT NULL,
      http_proxy_url text,
      https_proxy_url text,
      no_proxy text,
      start_time integer,
      team_id integer,
      state worker_state DEFAULT 'running'::worker_state NOT NULL,
      version text,
      CONSTRAINT addr_when_running CHECK ((((state <> 'stalled'::worker_state) AND (state <> 'landed'::worker_state) AND ((addr IS NOT NULL) OR (baggageclaim_url IS NOT NULL))) OR (((state = 'stalled'::worker_state) OR (state = 'landed'::worker_state)) AND (addr IS NULL) AND (baggageclaim_url IS NULL))))
  );



  ALTER TABLE ONLY base_resource_types ALTER COLUMN id SET DEFAULT nextval('base_resource_types_id_seq'::regclass);



  ALTER TABLE ONLY builds ALTER COLUMN id SET DEFAULT nextval('builds_id_seq'::regclass);



  ALTER TABLE ONLY containers ALTER COLUMN id SET DEFAULT nextval('containers_id_seq'::regclass);



  ALTER TABLE ONLY independent_build_inputs ALTER COLUMN id SET DEFAULT nextval('independent_build_inputs_id_seq'::regclass);



  ALTER TABLE ONLY jobs ALTER COLUMN id SET DEFAULT nextval('jobs_id_seq'::regclass);



  ALTER TABLE ONLY jobs_serial_groups ALTER COLUMN id SET DEFAULT nextval('jobs_serial_groups_id_seq'::regclass);



  ALTER TABLE ONLY next_build_inputs ALTER COLUMN id SET DEFAULT nextval('next_build_inputs_id_seq'::regclass);



  ALTER TABLE ONLY pipelines ALTER COLUMN id SET DEFAULT nextval('pipelines_id_seq'::regclass);



  ALTER TABLE ONLY resource_caches ALTER COLUMN id SET DEFAULT nextval('resource_caches_id_seq'::regclass);



  ALTER TABLE ONLY resource_config_check_sessions ALTER COLUMN id SET DEFAULT nextval('resource_config_check_sessions_id_seq'::regclass);



  ALTER TABLE ONLY resource_configs ALTER COLUMN id SET DEFAULT nextval('resource_configs_id_seq'::regclass);



  ALTER TABLE ONLY resource_types ALTER COLUMN id SET DEFAULT nextval('resource_types_id_seq'::regclass);



  ALTER TABLE ONLY resources ALTER COLUMN id SET DEFAULT nextval('resources_id_seq'::regclass);



  ALTER TABLE ONLY teams ALTER COLUMN id SET DEFAULT nextval('teams_id_seq'::regclass);



  ALTER TABLE ONLY versioned_resources ALTER COLUMN id SET DEFAULT nextval('versioned_resources_id_seq'::regclass);



  ALTER TABLE ONLY volumes ALTER COLUMN id SET DEFAULT nextval('volumes_id_seq'::regclass);



  ALTER TABLE ONLY worker_base_resource_types ALTER COLUMN id SET DEFAULT nextval('worker_base_resource_types_id_seq'::regclass);



  ALTER TABLE ONLY worker_resource_caches ALTER COLUMN id SET DEFAULT nextval('worker_resource_caches_id_seq'::regclass);



  ALTER TABLE ONLY worker_resource_config_check_sessions ALTER COLUMN id SET DEFAULT nextval('worker_resource_config_check_sessions_id_seq'::regclass);



  ALTER TABLE ONLY worker_task_caches ALTER COLUMN id SET DEFAULT nextval('worker_task_caches_id_seq'::regclass);



  ALTER TABLE ONLY base_resource_types
      ADD CONSTRAINT base_resource_types_name_key UNIQUE (name);



  ALTER TABLE ONLY base_resource_types
      ADD CONSTRAINT base_resource_types_pkey PRIMARY KEY (id);



  ALTER TABLE ONLY builds
      ADD CONSTRAINT builds_pkey PRIMARY KEY (id);



  ALTER TABLE ONLY workers
      ADD CONSTRAINT constraint_workers_name_unique UNIQUE (name);



  ALTER TABLE ONLY containers
      ADD CONSTRAINT containers_handle_key UNIQUE (handle);



  ALTER TABLE ONLY containers
      ADD CONSTRAINT containers_pkey PRIMARY KEY (id);



  ALTER TABLE ONLY independent_build_inputs
      ADD CONSTRAINT independent_build_inputs_pkey PRIMARY KEY (id);



  ALTER TABLE ONLY independent_build_inputs
      ADD CONSTRAINT independent_build_inputs_unique_job_id_input_name UNIQUE (job_id, input_name);



  ALTER TABLE ONLY jobs
      ADD CONSTRAINT jobs_pkey PRIMARY KEY (id);



  ALTER TABLE ONLY jobs_serial_groups
      ADD CONSTRAINT jobs_serial_groups_pkey PRIMARY KEY (id);



  ALTER TABLE ONLY jobs
      ADD CONSTRAINT jobs_unique_pipeline_id_name UNIQUE (pipeline_id, name);



  ALTER TABLE ONLY next_build_inputs
      ADD CONSTRAINT next_build_inputs_pkey PRIMARY KEY (id);



  ALTER TABLE ONLY next_build_inputs
      ADD CONSTRAINT next_build_inputs_unique_job_id_input_name UNIQUE (job_id, input_name);



  ALTER TABLE ONLY pipelines
      ADD CONSTRAINT pipelines_name_team_id UNIQUE (name, team_id);



  ALTER TABLE ONLY pipelines
      ADD CONSTRAINT pipelines_pkey PRIMARY KEY (id);



  ALTER TABLE ONLY pipes
      ADD CONSTRAINT pipes_pkey PRIMARY KEY (id);



  ALTER TABLE ONLY resource_caches
      ADD CONSTRAINT resource_caches_pkey PRIMARY KEY (id);



  ALTER TABLE ONLY resource_caches
      ADD CONSTRAINT resource_caches_resource_config_id_version_params_hash_key UNIQUE (resource_config_id, version, params_hash);



  ALTER TABLE ONLY resource_config_check_sessions
      ADD CONSTRAINT resource_config_check_sessions_pkey PRIMARY KEY (id);



  ALTER TABLE ONLY resource_configs
      ADD CONSTRAINT resource_configs_pkey PRIMARY KEY (id);



  ALTER TABLE ONLY resource_configs
      ADD CONSTRAINT resource_configs_resource_cache_id_base_resource_type_id_so_key UNIQUE (resource_cache_id, base_resource_type_id, source_hash);



  ALTER TABLE ONLY resource_types
      ADD CONSTRAINT resource_types_pipeline_id_name_key UNIQUE (pipeline_id, name);



  ALTER TABLE ONLY resource_types
      ADD CONSTRAINT resource_types_pkey PRIMARY KEY (id);



  ALTER TABLE ONLY resources
      ADD CONSTRAINT resources_pkey PRIMARY KEY (id);



  ALTER TABLE ONLY teams
      ADD CONSTRAINT teams_pkey PRIMARY KEY (id);



  ALTER TABLE ONLY resources
      ADD CONSTRAINT unique_pipeline_id_name UNIQUE (pipeline_id, name);



  ALTER TABLE ONLY versioned_resources
      ADD CONSTRAINT versioned_resources_pkey PRIMARY KEY (id);



  ALTER TABLE ONLY volumes
      ADD CONSTRAINT volumes_id_state_key UNIQUE (id, state);



  ALTER TABLE ONLY volumes
      ADD CONSTRAINT volumes_pkey PRIMARY KEY (id);



  ALTER TABLE ONLY volumes
      ADD CONSTRAINT volumes_worker_name_handle_key UNIQUE (worker_name, handle);



  ALTER TABLE ONLY worker_base_resource_types
      ADD CONSTRAINT worker_base_resource_types_pkey PRIMARY KEY (id);



  ALTER TABLE ONLY worker_base_resource_types
      ADD CONSTRAINT worker_base_resource_types_worker_name_base_resource_type_i_key UNIQUE (worker_name, base_resource_type_id);



  ALTER TABLE ONLY worker_resource_caches
      ADD CONSTRAINT worker_resource_caches_pkey PRIMARY KEY (id);



  ALTER TABLE ONLY worker_resource_config_check_sessions
      ADD CONSTRAINT worker_resource_config_check_sessions_pkey PRIMARY KEY (id);



  ALTER TABLE ONLY worker_task_caches
      ADD CONSTRAINT worker_task_caches_pkey PRIMARY KEY (id);



  ALTER TABLE ONLY workers
      ADD CONSTRAINT workers_addr_key UNIQUE (addr);



  ALTER TABLE ONLY workers
      ADD CONSTRAINT workers_pkey PRIMARY KEY (name);



  CREATE UNIQUE INDEX build_events_build_id_event_id ON build_events USING btree (build_id, event_id);



  CREATE INDEX build_events_build_id_idx ON build_events USING btree (build_id);



  CREATE INDEX build_image_resource_caches_build_id ON build_image_resource_caches USING btree (build_id);



  CREATE INDEX build_image_resource_caches_resource_cache_id ON build_image_resource_caches USING btree (resource_cache_id);



  CREATE INDEX build_inputs_build_id_idx ON build_inputs USING btree (build_id);



  CREATE INDEX build_inputs_build_id_versioned_resource_id ON build_inputs USING btree (build_id, versioned_resource_id);



  CREATE INDEX build_inputs_versioned_resource_id_idx ON build_inputs USING btree (versioned_resource_id);



  CREATE INDEX build_outputs_build_id_idx ON build_outputs USING btree (build_id);



  CREATE INDEX build_outputs_build_id_versioned_resource_id ON build_outputs USING btree (build_id, versioned_resource_id);



  CREATE INDEX build_outputs_versioned_resource_id_idx ON build_outputs USING btree (versioned_resource_id);



  CREATE INDEX builds_interceptible_completed ON builds USING btree (interceptible, completed);



  CREATE INDEX builds_job_id ON builds USING btree (job_id);



  CREATE INDEX builds_pipeline_id ON builds USING btree (pipeline_id);



  CREATE INDEX builds_status ON builds USING btree (status);



  CREATE INDEX builds_team_id ON builds USING btree (team_id);



  CREATE INDEX containers_build_id ON containers USING btree (build_id);



  CREATE INDEX containers_image_check_container_id ON containers USING btree (image_check_container_id);



  CREATE INDEX containers_image_get_container_id ON containers USING btree (image_get_container_id);



  CREATE INDEX containers_plan_id ON containers USING btree (plan_id);



  CREATE INDEX containers_team_id ON containers USING btree (team_id);



  CREATE INDEX containers_worker_name ON containers USING btree (worker_name);



  CREATE INDEX containers_worker_resource_config_check_session_id ON containers USING btree (worker_resource_config_check_session_id);



  CREATE INDEX independent_build_inputs_job_id ON independent_build_inputs USING btree (job_id);



  CREATE INDEX independent_build_inputs_version_id ON independent_build_inputs USING btree (version_id);



  CREATE UNIQUE INDEX index_teams_name_unique_case_insensitive ON teams USING btree (lower(name));



  CREATE INDEX jobs_pipeline_id ON jobs USING btree (pipeline_id);



  CREATE INDEX jobs_serial_groups_job_id_idx ON jobs_serial_groups USING btree (job_id);



  CREATE UNIQUE INDEX latest_completed_builds_per_job_id ON latest_completed_builds_per_job USING btree (id);



  CREATE INDEX next_build_inputs_job_id ON next_build_inputs USING btree (job_id);



  CREATE INDEX next_build_inputs_version_id ON next_build_inputs USING btree (version_id);



  CREATE UNIQUE INDEX next_builds_per_job_id ON next_builds_per_job USING btree (id);



  CREATE INDEX pipelines_team_id ON pipelines USING btree (team_id);



  CREATE INDEX pipes_team_id ON pipes USING btree (team_id);



  CREATE INDEX resource_cache_uses_build_id ON resource_cache_uses USING btree (build_id);



  CREATE INDEX resource_cache_uses_container_id ON resource_cache_uses USING btree (container_id);



  CREATE INDEX resource_cache_uses_resource_cache_id ON resource_cache_uses USING btree (resource_cache_id);



  CREATE INDEX resource_caches_resource_config_id ON resource_caches USING btree (resource_config_id);



  CREATE INDEX resource_config_check_sessions_resource_config_id ON resource_config_check_sessions USING btree (resource_config_id);



  CREATE INDEX resource_configs_base_resource_type_id ON resource_configs USING btree (base_resource_type_id);



  CREATE INDEX resource_configs_resource_cache_id ON resource_configs USING btree (resource_cache_id);



  CREATE INDEX resource_types_pipeline_id ON resource_types USING btree (pipeline_id);



  CREATE INDEX resource_types_resource_config_id ON resource_types USING btree (resource_config_id);



  CREATE INDEX resources_pipeline_id ON resources USING btree (pipeline_id);



  CREATE INDEX resources_resource_config_id ON resources USING btree (resource_config_id);



  CREATE UNIQUE INDEX transition_builds_per_job_id ON transition_builds_per_job USING btree (id);



  CREATE INDEX versioned_resources_resource_id_idx ON versioned_resources USING btree (resource_id);



  CREATE UNIQUE INDEX versioned_resources_resource_id_type_version ON versioned_resources USING btree (resource_id, type, md5(version));



  CREATE INDEX volumes_container_id ON volumes USING btree (container_id);



  CREATE UNIQUE INDEX volumes_handle ON volumes USING btree (handle);



  CREATE INDEX volumes_parent_id ON volumes USING btree (parent_id);



  CREATE INDEX volumes_team_id ON volumes USING btree (team_id);



  CREATE INDEX volumes_worker_base_resource_type_id ON volumes USING btree (worker_base_resource_type_id);



  CREATE INDEX volumes_worker_name ON volumes USING btree (worker_name);



  CREATE INDEX volumes_worker_resource_cache_id ON volumes USING btree (worker_resource_cache_id);



  CREATE UNIQUE INDEX volumes_worker_resource_cache_unique ON volumes USING btree (worker_resource_cache_id);



  CREATE INDEX volumes_worker_task_cache_id ON volumes USING btree (worker_task_cache_id);



  CREATE INDEX worker_base_resource_types_base_resource_type_id ON worker_base_resource_types USING btree (base_resource_type_id);



  CREATE INDEX worker_base_resource_types_worker_name ON worker_base_resource_types USING btree (worker_name);



  CREATE INDEX worker_resource_caches_resource_cache_id ON worker_resource_caches USING btree (resource_cache_id);



  CREATE INDEX worker_resource_caches_worker_base_resource_type_id ON worker_resource_caches USING btree (worker_base_resource_type_id);



  CREATE INDEX worker_resource_config_check_sessions_resource_config_check_ses ON worker_resource_config_check_sessions USING btree (resource_config_check_session_id);



  CREATE INDEX worker_resource_config_check_sessions_worker_base_resource_type ON worker_resource_config_check_sessions USING btree (worker_base_resource_type_id);



  CREATE INDEX worker_task_caches_job_id ON worker_task_caches USING btree (job_id);



  CREATE INDEX worker_task_caches_worker_name ON worker_task_caches USING btree (worker_name);



  CREATE INDEX workers_team_id ON workers USING btree (team_id);



  ALTER TABLE ONLY build_image_resource_caches
      ADD CONSTRAINT build_image_resource_caches_build_id_fkey FOREIGN KEY (build_id) REFERENCES builds(id) ON DELETE CASCADE;



  ALTER TABLE ONLY build_image_resource_caches
      ADD CONSTRAINT build_image_resource_caches_resource_cache_id_fkey FOREIGN KEY (resource_cache_id) REFERENCES resource_caches(id) ON DELETE RESTRICT;



  ALTER TABLE ONLY build_inputs
      ADD CONSTRAINT build_inputs_build_id_fkey FOREIGN KEY (build_id) REFERENCES builds(id) ON DELETE CASCADE;



  ALTER TABLE ONLY build_inputs
      ADD CONSTRAINT build_inputs_versioned_resource_id_fkey FOREIGN KEY (versioned_resource_id) REFERENCES versioned_resources(id) ON DELETE CASCADE;



  ALTER TABLE ONLY build_outputs
      ADD CONSTRAINT build_outputs_build_id_fkey FOREIGN KEY (build_id) REFERENCES builds(id) ON DELETE CASCADE;



  ALTER TABLE ONLY build_outputs
      ADD CONSTRAINT build_outputs_versioned_resource_id_fkey FOREIGN KEY (versioned_resource_id) REFERENCES versioned_resources(id) ON DELETE CASCADE;



  ALTER TABLE ONLY builds
      ADD CONSTRAINT builds_pipeline_id_fkey FOREIGN KEY (pipeline_id) REFERENCES pipelines(id) ON DELETE CASCADE;



  ALTER TABLE ONLY builds
      ADD CONSTRAINT builds_team_id_fkey FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE;



  ALTER TABLE ONLY containers
      ADD CONSTRAINT containers_build_id_fkey FOREIGN KEY (build_id) REFERENCES builds(id) ON DELETE SET NULL;



  ALTER TABLE ONLY containers
      ADD CONSTRAINT containers_image_check_container_id_fkey FOREIGN KEY (image_check_container_id) REFERENCES containers(id) ON DELETE SET NULL;



  ALTER TABLE ONLY containers
      ADD CONSTRAINT containers_image_get_container_id_fkey FOREIGN KEY (image_get_container_id) REFERENCES containers(id) ON DELETE SET NULL;



  ALTER TABLE ONLY containers
      ADD CONSTRAINT containers_team_id_fkey FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE SET NULL;



  ALTER TABLE ONLY containers
      ADD CONSTRAINT containers_worker_name_fkey FOREIGN KEY (worker_name) REFERENCES workers(name) ON DELETE CASCADE;



  ALTER TABLE ONLY containers
      ADD CONSTRAINT containers_worker_resource_config_check_session_id_fkey FOREIGN KEY (worker_resource_config_check_session_id) REFERENCES worker_resource_config_check_sessions(id) ON DELETE SET NULL;



  ALTER TABLE ONLY volumes
      ADD CONSTRAINT fkey_container_id FOREIGN KEY (container_id) REFERENCES containers(id) ON DELETE SET NULL;



  ALTER TABLE ONLY jobs_serial_groups
      ADD CONSTRAINT fkey_job_id FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE;



  ALTER TABLE ONLY builds
      ADD CONSTRAINT fkey_job_id FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE;



  ALTER TABLE ONLY versioned_resources
      ADD CONSTRAINT fkey_resource_id FOREIGN KEY (resource_id) REFERENCES resources(id) ON DELETE CASCADE;



  ALTER TABLE ONLY independent_build_inputs
      ADD CONSTRAINT independent_build_inputs_job_id_fkey FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE;



  ALTER TABLE ONLY independent_build_inputs
      ADD CONSTRAINT independent_build_inputs_version_id_fkey FOREIGN KEY (version_id) REFERENCES versioned_resources(id) ON DELETE CASCADE;



  ALTER TABLE ONLY jobs
      ADD CONSTRAINT jobs_pipeline_id_fkey FOREIGN KEY (pipeline_id) REFERENCES pipelines(id) ON DELETE CASCADE;



  ALTER TABLE ONLY next_build_inputs
      ADD CONSTRAINT next_build_inputs_job_id_fkey FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE;



  ALTER TABLE ONLY next_build_inputs
      ADD CONSTRAINT next_build_inputs_version_id_fkey FOREIGN KEY (version_id) REFERENCES versioned_resources(id) ON DELETE CASCADE;



  ALTER TABLE ONLY pipelines
      ADD CONSTRAINT pipelines_team_id_fkey FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE;



  ALTER TABLE ONLY pipes
      ADD CONSTRAINT pipes_team_id_fkey FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE;



  ALTER TABLE ONLY resource_cache_uses
      ADD CONSTRAINT resource_cache_uses_build_id_fkey FOREIGN KEY (build_id) REFERENCES builds(id) ON DELETE CASCADE;



  ALTER TABLE ONLY resource_cache_uses
      ADD CONSTRAINT resource_cache_uses_container_id_fkey FOREIGN KEY (container_id) REFERENCES containers(id) ON DELETE CASCADE;



  ALTER TABLE ONLY resource_cache_uses
      ADD CONSTRAINT resource_cache_uses_resource_cache_id_fkey FOREIGN KEY (resource_cache_id) REFERENCES resource_caches(id) ON DELETE RESTRICT;



  ALTER TABLE ONLY resource_caches
      ADD CONSTRAINT resource_caches_resource_config_id_fkey FOREIGN KEY (resource_config_id) REFERENCES resource_configs(id) ON DELETE RESTRICT;



  ALTER TABLE ONLY resource_config_check_sessions
      ADD CONSTRAINT resource_config_check_sessions_resource_config_id_fkey FOREIGN KEY (resource_config_id) REFERENCES resource_configs(id) ON DELETE RESTRICT;



  ALTER TABLE ONLY resource_configs
      ADD CONSTRAINT resource_configs_base_resource_type_id_fkey FOREIGN KEY (base_resource_type_id) REFERENCES base_resource_types(id) ON DELETE CASCADE;



  ALTER TABLE ONLY resource_configs
      ADD CONSTRAINT resource_configs_resource_cache_id_fkey FOREIGN KEY (resource_cache_id) REFERENCES resource_caches(id) ON DELETE RESTRICT;



  ALTER TABLE ONLY resource_types
      ADD CONSTRAINT resource_types_pipeline_id_fkey FOREIGN KEY (pipeline_id) REFERENCES pipelines(id) ON DELETE CASCADE;



  ALTER TABLE ONLY resource_types
      ADD CONSTRAINT resource_types_resource_config_id_fkey FOREIGN KEY (resource_config_id) REFERENCES resource_configs(id) ON DELETE SET NULL;



  ALTER TABLE ONLY resources
      ADD CONSTRAINT resources_pipeline_id_fkey FOREIGN KEY (pipeline_id) REFERENCES pipelines(id) ON DELETE CASCADE;



  ALTER TABLE ONLY resources
      ADD CONSTRAINT resources_resource_config_id_fkey FOREIGN KEY (resource_config_id) REFERENCES resource_configs(id) ON DELETE SET NULL;



  ALTER TABLE ONLY volumes
      ADD CONSTRAINT volumes_parent_id_fkey FOREIGN KEY (parent_id, parent_state) REFERENCES volumes(id, state) ON DELETE RESTRICT;



  ALTER TABLE ONLY volumes
      ADD CONSTRAINT volumes_team_id_fkey FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE SET NULL;



  ALTER TABLE ONLY volumes
      ADD CONSTRAINT volumes_worker_base_resource_type_id_fkey FOREIGN KEY (worker_base_resource_type_id) REFERENCES worker_base_resource_types(id) ON DELETE SET NULL;



  ALTER TABLE ONLY volumes
      ADD CONSTRAINT volumes_worker_name_fkey FOREIGN KEY (worker_name) REFERENCES workers(name) ON DELETE CASCADE;



  ALTER TABLE ONLY volumes
      ADD CONSTRAINT volumes_worker_resource_cache_id_fkey FOREIGN KEY (worker_resource_cache_id) REFERENCES worker_resource_caches(id) ON DELETE SET NULL;



  ALTER TABLE ONLY volumes
      ADD CONSTRAINT volumes_worker_task_cache_id_fkey FOREIGN KEY (worker_task_cache_id) REFERENCES worker_task_caches(id) ON DELETE SET NULL;



  ALTER TABLE ONLY worker_base_resource_types
      ADD CONSTRAINT worker_base_resource_types_base_resource_type_id_fkey FOREIGN KEY (base_resource_type_id) REFERENCES base_resource_types(id) ON DELETE RESTRICT;



  ALTER TABLE ONLY worker_base_resource_types
      ADD CONSTRAINT worker_base_resource_types_worker_name_fkey FOREIGN KEY (worker_name) REFERENCES workers(name) ON DELETE CASCADE;



  ALTER TABLE ONLY worker_resource_caches
      ADD CONSTRAINT worker_resource_caches_resource_cache_id_fkey FOREIGN KEY (resource_cache_id) REFERENCES resource_caches(id) ON DELETE CASCADE;



  ALTER TABLE ONLY worker_resource_caches
      ADD CONSTRAINT worker_resource_caches_worker_base_resource_type_id_fkey FOREIGN KEY (worker_base_resource_type_id) REFERENCES worker_base_resource_types(id) ON DELETE CASCADE;



  ALTER TABLE ONLY worker_resource_config_check_sessions
      ADD CONSTRAINT worker_resource_config_check__resource_config_check_sessio_fkey FOREIGN KEY (resource_config_check_session_id) REFERENCES resource_config_check_sessions(id) ON DELETE CASCADE;



  ALTER TABLE ONLY worker_resource_config_check_sessions
      ADD CONSTRAINT worker_resource_config_check__worker_base_resource_type_id_fkey FOREIGN KEY (worker_base_resource_type_id) REFERENCES worker_base_resource_types(id) ON DELETE CASCADE;



  ALTER TABLE ONLY worker_resource_config_check_sessions
      ADD CONSTRAINT worker_resource_config_check_sessions_team_id_fkey FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE;



  ALTER TABLE ONLY worker_task_caches
      ADD CONSTRAINT worker_task_caches_job_id_fkey FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE;



  ALTER TABLE ONLY worker_task_caches
      ADD CONSTRAINT worker_task_caches_worker_name_fkey FOREIGN KEY (worker_name) REFERENCES workers(name) ON DELETE CASCADE;



  ALTER TABLE ONLY workers
      ADD CONSTRAINT workers_team_id_fkey FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE;


COMMIT;
