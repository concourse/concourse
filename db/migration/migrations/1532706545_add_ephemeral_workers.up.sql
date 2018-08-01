BEGIN;
  ALTER TABLE "workers" ADD COLUMN "ephemeral" boolean;
COMMIT;

