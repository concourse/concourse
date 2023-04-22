ALTER SEQUENCE resource_config_versions_id_seq AS bigint;

ALTER TABLE build_comments
    ALTER COLUMN build_id TYPE bigint;

ALTER TABLE successful_build_outputs
    ALTER COLUMN rerun_of TYPE bigint;

ALTER TABLE jobs
    ALTER COLUMN first_logged_build_id TYPE bigint;

ALTER TABLE containers
    ALTER COLUMN meta_build_id TYPE bigint;
