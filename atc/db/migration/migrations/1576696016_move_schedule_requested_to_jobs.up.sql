BEGIN;
  ALTER TABLE jobs
    ADD COLUMN schedule_requested timestamp with time zone DEFAULT '1970-01-01 00:00:00'::timestamp with time zone NOT NULL,
    ADD COLUMN last_scheduled timestamp with time zone DEFAULT '1970-01-01 00:00:00'::timestamp with time zone NOT NULL;

  ALTER TABLE pipelines
    DROP COLUMN schedule_requested,
    DROP COLUMN last_scheduled;
COMMIT;
