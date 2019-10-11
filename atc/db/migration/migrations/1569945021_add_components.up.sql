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
    ('collector', '30s'),
    ('reaper', '30s'),
    ('drainer', '30s');

COMMIT;
