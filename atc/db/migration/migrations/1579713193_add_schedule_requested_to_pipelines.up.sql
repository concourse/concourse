BEGIN;
  ALTER TABLE pipelines
    ADD COLUMN schedule_requested timestamp with time zone DEFAULT '1970-01-01 00:00:00'::timestamp with time zone NOT NULL;
COMMIT;
