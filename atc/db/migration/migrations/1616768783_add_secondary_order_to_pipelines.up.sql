ALTER TABLE pipelines ADD COLUMN secondary_ordering integer;

WITH s AS (
  SELECT id, row_number() OVER (PARTITION BY team_id, name ORDER BY id) as secondary_ordering
  FROM pipelines
)
UPDATE pipelines p
SET secondary_ordering = s.secondary_ordering
FROM s
WHERE p.id = s.id;

ALTER TABLE pipelines ALTER COLUMN secondary_ordering SET NOT NULL;
