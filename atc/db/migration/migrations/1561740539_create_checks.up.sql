BEGIN;

  CREATE TABLE checks (
      id serial PRIMARY KEY,
      resource_config_scope_id integer REFERENCES resource_config_scopes(id) ON DELETE CASCADE,
      resource_config_id integer REFERENCES resource_configs(id) ON DELETE CASCADE,
      base_resource_type_id integer REFERENCES base_resource_types(id) ON DELETE CASCADE,
      schema text NOT NULL,
      status text NOT NULL,
      plan text,
      nonce text,
      check_error text,
      create_time timestamp WITH TIME ZONE DEFAULT now() NOT NULL,
      start_time timestamp WITH TIME ZONE,
      end_time timestamp WITH TIME ZONE
  );

  CREATE UNIQUE INDEX resource_config_scope_id_key ON checks (resource_config_scope_id) WHERE status = 'started';

COMMIT;
