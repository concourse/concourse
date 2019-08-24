BEGIN;
  DROP TABLE build_pipes;

  DROP TABLE next_build_pipes;

  ALTER TABLE pipelines
    ADD COLUMN last_scheduled timestamp with time zone DEFAULT '1970-01-01 00:00:00'::timestamp with time zone NOT NULL;

  ALTER TABLE jobs
    DROP COLUMN last_scheduled;
COMMIT;
