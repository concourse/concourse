BEGIN;
  ALTER TABLE volumes DROP CONSTRAINT volumes_parent_id_fkey;

  ALTER TABLE volumes ADD CONSTRAINT volumes_parent_id_fkey
    FOREIGN KEY (parent_id, parent_state)
    REFERENCES volumes (id, state) ON DELETE NO ACTION DEFERRABLE;

  CREATE INDEX missing_volumes_idx ON volumes (state) WHERE missing_since IS NOT NULL;
COMMIT;
