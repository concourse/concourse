BEGIN;
  CREATE TABLE versioned_resources (
      id integer NOT NULL,
      version text NOT NULL,
      metadata text NOT NULL,
      type text NOT NULL,
      enabled boolean DEFAULT true NOT NULL,
      resource_id integer,
      check_order integer DEFAULT 0 NOT NULL
  );

  CREATE SEQUENCE versioned_resources_id_seq
      START WITH 1
      INCREMENT BY 1
      NO MINVALUE
      NO MAXVALUE
      CACHE 1;

  ALTER SEQUENCE versioned_resources_id_seq OWNED BY versioned_resources.id;

  ALTER TABLE ONLY versioned_resources ALTER COLUMN id SET DEFAULT nextval('versioned_resources_id_seq'::regclass);

  ALTER TABLE ONLY versioned_resources
    ADD CONSTRAINT versioned_resources_pkey PRIMARY KEY (id);

  CREATE INDEX versioned_resources_resource_id_idx ON versioned_resources USING btree (resource_id);

  CREATE UNIQUE INDEX versioned_resources_resource_id_type_version ON versioned_resources USING btree (resource_id, type, md5(version));

  ALTER TABLE ONLY versioned_resources
    ADD CONSTRAINT fkey_resource_id FOREIGN KEY (resource_id) REFERENCES resources(id) ON DELETE CASCADE;

  CREATE TABLE build_inputs (
      build_id integer,
      versioned_resource_id integer,
      name text NOT NULL
  );

  CREATE INDEX build_inputs_build_id_idx ON build_inputs USING btree (build_id);

  CREATE INDEX build_inputs_build_id_versioned_resource_id ON build_inputs USING btree (build_id, versioned_resource_id);

  CREATE INDEX build_inputs_versioned_resource_id_idx ON build_inputs USING btree (versioned_resource_id);

  ALTER TABLE ONLY build_inputs
      ADD CONSTRAINT build_inputs_build_id_fkey FOREIGN KEY (build_id) REFERENCES builds(id) ON DELETE CASCADE;

  ALTER TABLE ONLY build_inputs
      ADD CONSTRAINT build_inputs_versioned_resource_id_fkey FOREIGN KEY (versioned_resource_id) REFERENCES versioned_resources(id) ON DELETE CASCADE;

  CREATE TABLE build_outputs (
      build_id integer,
      versioned_resource_id integer,
      explicit boolean DEFAULT false NOT NULL
  );

  CREATE INDEX build_outputs_build_id_idx ON build_outputs USING btree (build_id);

  CREATE INDEX build_outputs_build_id_versioned_resource_id ON build_outputs USING btree (build_id, versioned_resource_id);

  CREATE INDEX build_outputs_versioned_resource_id_idx ON build_outputs USING btree (versioned_resource_id);

  ALTER TABLE ONLY build_outputs
      ADD CONSTRAINT build_outputs_build_id_fkey FOREIGN KEY (build_id) REFERENCES builds(id) ON DELETE CASCADE;

  ALTER TABLE ONLY build_outputs
      ADD CONSTRAINT build_outputs_versioned_resource_id_fkey FOREIGN KEY (versioned_resource_id) REFERENCES versioned_resources(id) ON DELETE CASCADE;

  ALTER TABLE resources
    ADD COLUMN paused boolean DEFAULT false;
COMMIT;
