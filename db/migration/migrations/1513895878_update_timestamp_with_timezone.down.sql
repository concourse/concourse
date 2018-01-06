BEGIN;
    ALTER TABLE build_inputs
      ALTER COLUMN modified_time TYPE timestamp without time zone;
COMMIT;

BEGIN;
    ALTER TABLE build_outputs
      ALTER COLUMN modified_time TYPE timestamp without time zone;
COMMIT;

BEGIN;
    ALTER TABLE cache_invalidator
      ALTER COLUMN last_invalidated TYPE timestamp without time zone;
    ALTER TABLE cache_invalidator
      ALTER COLUMN last_invalidated SET DEFAULT '1970-01-01 00:00:00'::timestamp without time zone;
COMMIT;

BEGIN;
    ALTER TABLE containers
      ALTER COLUMN best_if_used_by TYPE timestamp without time zone;
COMMIT;

BEGIN;
    ALTER TABLE pipelines
      ALTER COLUMN last_scheduled TYPE timestamp without time zone;
    ALTER TABLE pipelines
      ALTER COLUMN last_scheduled SET DEFAULT '1970-01-01 00:00:00'::timestamp without time zone;
COMMIT;

BEGIN;
    ALTER TABLE resource_types
      ALTER COLUMN last_checked TYPE timestamp without time zone;
    ALTER TABLE resource_types
      ALTER COLUMN last_checked SET DEFAULT '1970-01-01 00:00:00'::timestamp without time zone;
COMMIT;

BEGIN;
    ALTER TABLE resources
      ALTER COLUMN last_checked TYPE timestamp without time zone;
    ALTER TABLE resources
      ALTER COLUMN last_checked SET DEFAULT '1970-01-01 00:00:00'::timestamp without time zone;
COMMIT;

BEGIN;
    ALTER TABLE versioned_resources
      ALTER COLUMN modified_time TYPE timestamp without time zone;
COMMIT;
