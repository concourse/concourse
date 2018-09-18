BEGIN;
  ALTER TABLE "build_outputs" ADD COLUMN "explicit" boolean;
COMMIT;
