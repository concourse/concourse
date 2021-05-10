
DROP INDEX builds_resource_id_idx;

-- prevent conflicts when re-introducing unique constraint by nulling-out all
-- but the current check build for each resource
UPDATE builds b
SET resource_id = NULL
WHERE resource_id IS NOT NULL
AND NOT EXISTS (
  SELECT 1
  FROM resources
  WHERE build_id = b.id
);

CREATE UNIQUE INDEX builds_resource_id_key ON builds (resource_id);

ALTER TABLE resources DROP COLUMN build_id;

