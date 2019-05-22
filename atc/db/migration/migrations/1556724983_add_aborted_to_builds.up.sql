BEGIN;

  ALTER TABLE builds ADD COLUMN aborted boolean DEFAULT false NOT NULL;

  UPDATE builds SET aborted = true, completed = true WHERE status = 'aborted';

COMMIT;
