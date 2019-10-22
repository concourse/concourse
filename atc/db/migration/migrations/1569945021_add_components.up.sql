BEGIN;

  CREATE TABLE components (
      id serial PRIMARY KEY,
      name text NOT NULL,
      interval text NOT NULL,
      last_ran timestamp WITH TIME ZONE,
      paused boolean DEFAULT FALSE
  );

  CREATE UNIQUE INDEX components_name_key ON components (name);

COMMIT;
