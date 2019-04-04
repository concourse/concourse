BEGIN;
  CREATE TABLE resource_pins (
    resource_id INTEGER NOT NULL PRIMARY KEY
      REFERENCES resources(id) ON DELETE CASCADE,
    version jsonb NOT NULL,
    comment_text text NOT NULL
  );

  INSERT INTO resource_pins (
    SELECT
      id AS resource_id,
      api_pinned_version AS version,
      COALESCE (pin_comment, '') AS comment_text
    FROM resources
    WHERE api_pinned_version IS NOT NULL
  );

  ALTER TABLE resources
    DROP COLUMN api_pinned_version,
    DROP COLUMN pin_comment;
COMMIT;
