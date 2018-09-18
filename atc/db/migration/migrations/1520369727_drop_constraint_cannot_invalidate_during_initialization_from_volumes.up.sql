BEGIN;
  ALTER TABLE "volumes"
  DROP CONSTRAINT "cannot_invalidate_during_initialization";
COMMIT;
