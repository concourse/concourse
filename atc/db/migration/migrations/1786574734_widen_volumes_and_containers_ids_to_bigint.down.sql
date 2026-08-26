-- Reverse of the volumes/containers id-widening migration: bigint -> integer.
-- This can fail if a live sequence value already exceeds the 32-bit integer
-- range, which is the expected/standard limitation for narrowing back to
-- integer.
ALTER TABLE volumes DROP CONSTRAINT volumes_parent_id_fkey;

ALTER TABLE containers
    ALTER COLUMN id TYPE integer,
    ALTER COLUMN image_check_container_id TYPE integer,
    ALTER COLUMN image_get_container_id TYPE integer;

ALTER TABLE volumes
    ALTER COLUMN id TYPE integer,
    ALTER COLUMN parent_id TYPE integer,
    ALTER COLUMN container_id TYPE integer;

ALTER TABLE volumes ADD CONSTRAINT volumes_parent_id_fkey
    FOREIGN KEY (parent_id, parent_state)
    REFERENCES volumes (id, state) DEFERRABLE;
