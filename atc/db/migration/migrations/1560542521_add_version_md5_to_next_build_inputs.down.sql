BEGIN;
  TRUNCATE TABLE next_build_inputs;

  ALTER TABLE next_build_inputs
    ADD COLUMN resource_config_version_id integer NOT NULL REFERENCES resource_config_versions (id) ON DELETE CASCADE,
    ADD COLUMN id integer NOT NULL,
    DROP COLUMN version_md5;

  CREATE SEQUENCE next_build_inputs_id_seq
      START WITH 1
      INCREMENT BY 1
      NO MINVALUE
      NO MAXVALUE
      CACHE 1;

  ALTER SEQUENCE next_build_inputs_id_seq OWNED BY next_build_inputs.id;

  ALTER TABLE ONLY next_build_inputs ALTER COLUMN id SET DEFAULT nextval('next_build_inputs_id_seq'::regclass);

  ALTER TABLE ONLY next_build_inputs
      ADD CONSTRAINT next_build_inputs_pkey PRIMARY KEY (id);

  CREATE INDEX next_build_inputs_resource_config_version_id ON next_build_inputs (resource_config_version_id);

COMMIT;
