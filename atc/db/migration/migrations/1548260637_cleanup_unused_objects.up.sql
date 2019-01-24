BEGIN;
  DROP TYPE volume_state_old;
  DROP TYPE container_state_old;
  ALTER TABLE containers DROP COLUMN best_if_used_by;
COMMIT;
