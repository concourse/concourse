ALTER TABLE resource_cache_uses
    ADD COLUMN in_memory_build_id bigint,
    ADD COLUMN in_memory_build_create_time bigint;

CREATE INDEX resource_cache_uses_in_memory_build_id_idx ON resource_cache_uses (in_memory_build_id, in_memory_build_create_time);
