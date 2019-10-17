BEGIN;

  CREATE TABLE components (
      id serial PRIMARY KEY,
      name text NOT NULL,
      interval text NOT NULL,
      last_ran timestamp WITH TIME ZONE,
      paused boolean DEFAULT FALSE
  );

  CREATE UNIQUE INDEX components_name_key ON components (name);

  INSERT INTO components(name, interval) VALUES
    ('tracker', '10s'),
    ('scheduler', '10s'),
    ('scanner', '1m'),
    ('checker', '10s'),
    ('reaper', '30s'),
    ('drainer', '30s'),
    ('collector_builds', '30s'),
    ('collector_workers', '30s'),
    ('collector_resource_configs', '30s'),
    ('collector_resource_caches', '30s'),
    ('collector_resource_cache_uses', '30s'),
    ('collector_artifacts', '30s'),
    ('collector_checks', '30s'),
    ('collector_volumes', '30s'),
    ('collector_containers', '30s'),
    ('collector_check_sessions', '30s');

COMMIT;
