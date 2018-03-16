BEGIN;
  CREATE TABLE pipes (
      id text NOT NULL,
      url text,
      team_id integer NOT NULL
  );

  ALTER TABLE ONLY pipes
      ADD CONSTRAINT pipes_pkey PRIMARY KEY (id);

  CREATE INDEX pipes_team_id ON pipes USING btree (team_id);

  ALTER TABLE ONLY pipes
      ADD CONSTRAINT pipes_team_id_fkey FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE;
COMMIT;
