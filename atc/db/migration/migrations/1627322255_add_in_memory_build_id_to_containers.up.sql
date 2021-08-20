ALTER TABLE containers
    ADD COLUMN in_memory_check_build_id int8;

CREATE UNIQUE INDEX containers_in_memory_check_build_id_idx ON containers (in_memory_check_build_id);
