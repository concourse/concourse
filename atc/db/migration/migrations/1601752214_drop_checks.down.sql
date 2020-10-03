BEGIN;
  create table checks
  (
    id bigserial not null
    constraint checks_pkey
    primary key,
    resource_config_scope_id integer
    constraint checks_resource_config_scope_id_fkey
    references resource_config_scopes
    on delete cascade,
    schema text not null,
    status text not null,
    manually_triggered boolean default false,
    plan text,
    nonce text,
    check_error text,
    metadata jsonb,
    create_time timestamp with time zone default now() not null,
    start_time timestamp with time zone,
    end_time timestamp with time zone,
    span_context jsonb
  );

  create unique index resource_config_scope_id_key
  on checks (resource_config_scope_id)
  where ((status = 'started'::text) AND (manually_triggered = false));

  create index started_checks_idx
  on checks (id)
  where (status = 'started'::text);
COMMIT;
