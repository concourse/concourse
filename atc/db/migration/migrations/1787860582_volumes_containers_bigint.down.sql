ALTER TABLE containers
    ALTER COLUMN id TYPE int,
    ALTER COLUMN image_check_container_id TYPE int,
    ALTER COLUMN image_get_container_id TYPE int,
    ALTER COLUMN resource_config_check_session_id TYPE int;

ALTER TABLE volumes DROP CONSTRAINT volumes_parent_id_fkey;

ALTER TABLE volumes
    ALTER COLUMN id TYPE int,
    ALTER COLUMN parent_id TYPE int,
    ALTER COLUMN container_id TYPE int,
    ALTER COLUMN worker_task_cache_id TYPE int,
    ALTER COLUMN worker_resource_cache_id TYPE int;

ALTER TABLE volumes
    ADD CONSTRAINT volumes_parent_id_fkey
    FOREIGN KEY (parent_id, parent_state)
    REFERENCES volumes(id, state) DEFERRABLE;

ALTER TABLE resource_caches
    ALTER COLUMN id TYPE int;

ALTER TABLE resource_configs
    ALTER COLUMN resource_cache_id TYPE int;

ALTER TABLE build_image_resource_caches
    ALTER COLUMN resource_cache_id TYPE int;

ALTER TABLE resource_cache_uses
    ALTER COLUMN resource_cache_id TYPE int;

ALTER TABLE resource_config_check_sessions
    ALTER COLUMN id TYPE int;

ALTER TABLE task_caches
    ALTER COLUMN id TYPE int;
ALTER SEQUENCE task_caches_id_seq AS int;

ALTER TABLE worker_task_caches
    ALTER COLUMN id TYPE int,
    ALTER COLUMN task_cache_id TYPE int;

ALTER TABLE worker_resource_caches
    ALTER COLUMN id TYPE int,
    ALTER COLUMN resource_cache_id TYPE int;
