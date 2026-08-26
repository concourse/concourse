-- Widen the volumes and containers id columns (and the columns that reference
-- them through a foreign key) from integer to bigint, so their high-churn
-- sequences cannot overflow the 32-bit integer range and produce
-- "ERROR: integer out of range (SQLSTATE 22003)". See
-- https://github.com/concourse/concourse/issues/9672.
--
-- Only the volumes and containers tables are rewritten by this migration.
--
-- The volumes composite self-foreign-key (parent_id, parent_state) ->
-- (id, state) is dropped first: altering the id side otherwise fails with
-- "could not find cast from volume_state to anyenum". The volumes(id, state)
-- unique constraint it depends on is preserved throughout.
ALTER TABLE volumes DROP CONSTRAINT volumes_parent_id_fkey;

ALTER TABLE containers
    ALTER COLUMN id TYPE bigint,
    ALTER COLUMN image_check_container_id TYPE bigint,
    ALTER COLUMN image_get_container_id TYPE bigint;

ALTER TABLE volumes
    ALTER COLUMN id TYPE bigint,
    ALTER COLUMN parent_id TYPE bigint,
    ALTER COLUMN container_id TYPE bigint;

ALTER TABLE volumes ADD CONSTRAINT volumes_parent_id_fkey
    FOREIGN KEY (parent_id, parent_state)
    REFERENCES volumes (id, state) DEFERRABLE;
