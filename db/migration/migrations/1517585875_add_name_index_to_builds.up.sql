BEGIN;
  CREATE INDEX builds_name ON builds USING btree (name);
COMMIT;
