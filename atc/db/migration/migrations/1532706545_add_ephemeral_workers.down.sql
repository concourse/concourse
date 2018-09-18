BEGIN;
  ALTER TABLE "workers" DROP COLUMN "ephemeral";
COMMIT;

