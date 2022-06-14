ALTER SEQUENCE resource_config_versions_id_seq AS integer;

ALTER TABLE build_comments
    ALTER COLUMN build_id TYPE integer;

ALTER TABLE builds
    ALTER COLUMN rerun_of TYPE integer;

ALTER TABLE successful_build_outputs
    ALTER COLUMN rerun_of TYPE integer;

ALTER TABLE jobs
    ALTER COLUMN first_logged_build_id TYPE integer;

ALTER TABLE containers
    ALTER COLUMN meta_build_id TYPE integer;
