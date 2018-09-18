BEGIN;
  DELETE FROM "build_outputs" WHERE NOT "explicit";
  ALTER TABLE "build_outputs" DROP COLUMN "explicit";
COMMIT;
