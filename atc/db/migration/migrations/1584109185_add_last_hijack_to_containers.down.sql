BEGIN;
  ALTER TABLE containers DROP COLUMN last_hijack;

  ALTER TABLE containers ADD COLUMN hijacked boolean DEFAULT false NOT NULL,
  ALTER TABLE containers ADD COLUMN discontinued boolean DEFAULT false NOT NULL,
COMMIT;
