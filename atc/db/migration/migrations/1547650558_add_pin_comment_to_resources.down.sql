BEGIN;
  ALTER TABLE resources
    DROP COLUMN pin_comment;
COMMIT;
