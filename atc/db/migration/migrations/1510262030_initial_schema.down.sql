BEGIN;

  DROP MATERIALIZED VIEW
    latest_completed_builds_per_job,
    next_builds_per_job,
    transition_builds_per_job
    CASCADE;

  DROP SEQUENCE
    config_version_seq,
    one_off_name;

  DROP TABLE
    base_resource_types,
    build_events,
    build_image_resource_caches,
    build_inputs,
    build_outputs,
    builds,
    cache_invalidator,
    containers,
    independent_build_inputs,
    jobs,
    jobs_serial_groups,
    next_build_inputs,
    pipelines,
    pipes,
    resource_cache_uses,
    resource_caches,
    resource_config_check_sessions,
    resource_configs,
    resource_types,
    resources,
    teams,
    versioned_resources,
    volumes,
    worker_base_resource_types,
    worker_resource_caches,
    worker_resource_config_check_sessions,
    worker_task_caches,
    workers
    CASCADE;

  DROP TYPE
    build_status,
    container_state,
    container_stage,
    container_state_old,
    volume_state,
    volume_state_old,
    worker_state;

COMMIT;
