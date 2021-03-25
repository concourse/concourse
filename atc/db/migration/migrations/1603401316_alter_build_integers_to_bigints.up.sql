
ALTER TABLE jobs
    ALTER COLUMN next_build_id TYPE bigint,
    ALTER COLUMN latest_completed_build_id TYPE bigint,
    ALTER COLUMN transition_build_id TYPE bigint;

ALTER TABLE build_pipes
    ALTER COLUMN from_build_id TYPE bigint,
    ALTER COLUMN to_build_id TYPE bigint;

ALTER TABLE build_image_resource_caches
    ALTER COLUMN build_id TYPE bigint;

ALTER TABLE build_resource_config_version_inputs
    ALTER COLUMN build_id TYPE bigint;

ALTER TABLE build_resource_config_version_outputs
    ALTER COLUMN build_id TYPE bigint;

ALTER TABLE containers
    ALTER COLUMN build_id TYPE bigint;

ALTER TABLE next_build_pipes
    ALTER COLUMN from_build_id TYPE bigint;

ALTER TABLE pipelines
    ALTER COLUMN parent_build_id TYPE bigint;

ALTER TABLE resource_cache_uses
    ALTER COLUMN build_id TYPE bigint;

ALTER TABLE successful_build_outputs
    ALTER COLUMN build_id TYPE bigint;

ALTER TABLE worker_artifacts
    ALTER COLUMN build_id TYPE bigint;

