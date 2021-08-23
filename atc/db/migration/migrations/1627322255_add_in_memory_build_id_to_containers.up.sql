ALTER TABLE containers
    ADD COLUMN in_memory_build_id bigint;

CREATE INDEX containers_in_memory_build_id_idx ON containers (in_memory_build_id);
