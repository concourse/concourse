BEGIN;
  -- Need to lock builds as migrating foreign key contraints seems to acquire an
  -- AccessExclusiveLock on the downstream table. Locks are released at the end
  -- of the transaction
  LOCK TABLE builds IN ACCESS EXCLUSIVE MODE;

  ALTER TABLE jobs
      ALTER COLUMN next_build_id TYPE integer,
      ALTER COLUMN latest_completed_build_id TYPE integer,
      ALTER COLUMN transition_build_id TYPE integer;

  ALTER TABLE build_pipes
      ALTER COLUMN from_build_id TYPE integer,
      ALTER COLUMN to_build_id TYPE integer;

  ALTER TABLE build_image_resource_caches
      ALTER COLUMN build_id TYPE integer;

  ALTER TABLE build_resource_config_version_inputs
      ALTER COLUMN build_id TYPE integer;

  ALTER TABLE build_resource_config_version_outputs
      ALTER COLUMN build_id TYPE integer;

  ALTER TABLE containers
      ALTER COLUMN build_id TYPE integer;

  ALTER TABLE next_build_pipes
      ALTER COLUMN from_build_id TYPE integer;

  ALTER TABLE pipelines
      ALTER COLUMN parent_build_id TYPE integer;

  ALTER TABLE resource_cache_uses
      ALTER COLUMN build_id TYPE integer;

  ALTER TABLE successful_build_outputs
      ALTER COLUMN build_id TYPE integer;

  ALTER TABLE worker_artifacts
      ALTER COLUMN build_id TYPE integer;
COMMIT;
