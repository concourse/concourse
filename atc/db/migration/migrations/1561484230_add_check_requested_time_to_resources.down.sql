BEGIN;
  ALTER TABLE resources
    DROP COLUMN "check_requested_time";

  ALTER TABLE resource_types
    DROP COLUMN "check_requested_time";
END;
