BEGIN;
  ALTER TABLE resources
    ADD COLUMN "check_requested_time" timestamp with time zone NOT NULL DEFAULT '1970-01-01 00:00:00';

  ALTER TABLE resource_types
    ADD COLUMN "check_requested_time" timestamp with time zone NOT NULL DEFAULT '1970-01-01 00:00:00';
END;
