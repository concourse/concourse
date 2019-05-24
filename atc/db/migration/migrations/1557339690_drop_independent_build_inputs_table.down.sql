BEGIN;
  CREATE TABLE independent_build_inputs (
      id integer NOT NULL,
      job_id integer NOT NULL,
      input_name text NOT NULL,
      first_occurrence boolean NOT NULL,
      resource_config_version_id integer NOT NULL REFERENCES resource_config_versions (id) ON DELETE CASCADE,
      resource_id integer NOT NULL REFERENCES resources (id) ON DELETE CASCADE
  );

  CREATE SEQUENCE independent_build_inputs_id_seq
      START WITH 1
      INCREMENT BY 1
      NO MINVALUE
      NO MAXVALUE
      CACHE 1;

  ALTER SEQUENCE independent_build_inputs_id_seq OWNED BY independent_build_inputs.id;

  ALTER TABLE ONLY independent_build_inputs ALTER COLUMN id SET DEFAULT nextval('independent_build_inputs_id_seq'::regclass);

  ALTER TABLE ONLY independent_build_inputs
      ADD CONSTRAINT independent_build_inputs_pkey PRIMARY KEY (id);

  ALTER TABLE ONLY independent_build_inputs
      ADD CONSTRAINT independent_build_inputs_unique_job_id_input_name UNIQUE (job_id, input_name);

  CREATE INDEX independent_build_inputs_resource_config_version_id ON independent_build_inputs (resource_config_version_id);

  CREATE INDEX independent_build_inputs_job_id ON independent_build_inputs USING btree (job_id);

  ALTER TABLE ONLY independent_build_inputs
      ADD CONSTRAINT independent_build_inputs_job_id_fkey FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE;
COMMIT;
