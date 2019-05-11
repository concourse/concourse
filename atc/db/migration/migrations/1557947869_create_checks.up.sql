BEGIN;

  CREATE TABLE checks (
      id integer,
      resource_config_scope_id integer REFERENCES resource_config_scopes(id) ON DELETE CASCADE,
      create_time timestamp without time zone DEFAULT now() NOT NULL,
      start_time timestamp without time zone,
      end_time timestamp without time zone,
      from_version jsonb
  );

COMMIT;
