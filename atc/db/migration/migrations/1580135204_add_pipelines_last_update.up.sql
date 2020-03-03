BEGIN;
  ALTER TABLE pipelines
    ADD COLUMN "last_updated" timestamp with time zone NOT NULL DEFAULT '1970-01-01 00:00:00';
COMMIT;
