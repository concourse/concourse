BEGIN;

  CREATE TABLE checks (
      id bigserial PRIMARY KEY,
      resource_config_scope_id integer REFERENCES resource_config_scopes(id) ON DELETE CASCADE,
      schema text NOT NULL,
      status text NOT NULL,
      manually_triggered boolean DEFAULT false,
      plan text,
      nonce text,
      check_error text,
      metadata jsonb,
      create_time timestamp WITH TIME ZONE DEFAULT now() NOT NULL,
      start_time timestamp WITH TIME ZONE,
      end_time timestamp WITH TIME ZONE
  );

  CREATE UNIQUE INDEX resource_config_scope_id_key ON checks (resource_config_scope_id) WHERE status = 'started' AND manually_triggered = false;

COMMIT;
