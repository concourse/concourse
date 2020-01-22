BEGIN;
  ALTER TABLE volumes DROP CONSTRAINT volumes_parent_id_fkey;

  ALTER TABLE ONLY volumes
      ADD CONSTRAINT volumes_parent_id_fkey FOREIGN KEY (parent_id, parent_state) REFERENCES volumes(id, state) ON DELETE RESTRICT;

  DROP INDEX missing_volumes_idx;
COMMIT;
