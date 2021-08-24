ALTER TABLE containers
    ADD COLUMN in_memory_build_id bigint,
    ADD COLUMN in_memory_build_create_time bigint;

CREATE INDEX containers_in_memory_build_id_idx ON containers (in_memory_build_id, in_memory_build_create_time);
