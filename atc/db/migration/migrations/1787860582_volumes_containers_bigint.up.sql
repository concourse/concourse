ALTER TABLE containers
    ALTER COLUMN id TYPE bigint,
    ALTER COLUMN image_check_container_id TYPE bigint,
    ALTER COLUMN image_get_container_id TYPE bigint,
    ALTER COLUMN resource_config_check_session_id TYPE bigint;

-- We usually don't need to drop constraints, but we found in this case we ran
-- into a psql bug if we ran the query altering the volumes table multiple
-- times. We generally expect these queries to be idempotent, so dropping and
-- adding back the constraint make this migration idempotent.
ALTER TABLE volumes DROP CONSTRAINT volumes_parent_id_fkey;

ALTER TABLE volumes
    ALTER COLUMN id TYPE bigint,
    ALTER COLUMN parent_id TYPE bigint,
    ALTER COLUMN container_id TYPE bigint,
    ALTER COLUMN worker_task_cache_id TYPE bigint,
    ALTER COLUMN worker_resource_cache_id TYPE bigint;

ALTER TABLE volumes
    ADD CONSTRAINT volumes_parent_id_fkey
    FOREIGN KEY (parent_id, parent_state)
    REFERENCES volumes(id, state) DEFERRABLE;

ALTER TABLE resource_caches
    ALTER COLUMN id TYPE bigint;

ALTER TABLE resource_configs
    ALTER COLUMN resource_cache_id TYPE bigint;

ALTER TABLE build_image_resource_caches
    ALTER COLUMN resource_cache_id TYPE bigint;

ALTER TABLE resource_cache_uses
    ALTER COLUMN resource_cache_id TYPE bigint;

ALTER TABLE resource_config_check_sessions
    ALTER COLUMN id TYPE bigint;

ALTER TABLE task_caches
    ALTER COLUMN id TYPE bigint;
-- The task_caches.id column was made with 'id serial' which results in the
-- sequence having a max value set to the int4 max.
-- Other tables did 'id integer' during creation so the sequence backing their
-- id columns don't have a max value defined
ALTER SEQUENCE task_caches_id_seq AS bigint;

ALTER TABLE worker_task_caches
    ALTER COLUMN id TYPE bigint,
    ALTER COLUMN task_cache_id TYPE bigint;

ALTER TABLE worker_resource_caches
    ALTER COLUMN id TYPE bigint,
    ALTER COLUMN resource_cache_id TYPE bigint;
