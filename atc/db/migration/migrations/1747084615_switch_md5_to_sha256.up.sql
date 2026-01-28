-- Ensure the pgcrypto extension is available for hashing
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Keep version_md5 so the scheduler/algorithm planning keeps working with
-- historical data
ALTER TABLE resource_config_versions
ADD COLUMN version_sha256 TEXT;

-- Temporary check constraint to avoid full table scan when we add NOT NULL later
ALTER TABLE resource_config_versions
ADD CONSTRAINT temporary_digest_not_null
CHECK (version_sha256 IS NOT NULL) NOT VALID;

-- Rename columns from version_md5 to version_digest for all other tables. Other
-- tables will now have a mix of md5 and sha256 digests
ALTER TABLE build_resource_config_version_inputs
RENAME COLUMN version_md5 TO version_digest;

ALTER TABLE build_resource_config_version_outputs
RENAME COLUMN version_md5 TO version_digest;

ALTER TABLE next_build_inputs
RENAME COLUMN version_md5 TO version_digest;

ALTER TABLE resource_caches
RENAME COLUMN version_md5 TO version_digest;

ALTER TABLE resource_disabled_versions
RENAME COLUMN version_md5 TO version_digest;

-- Drop Indexes

-- ensure lookups on md5 digests are still fast by preseving the index
CREATE INDEX resource_config_versions_version_md5
ON resource_config_versions (version_md5);

ALTER TABLE resource_config_versions
DROP CONSTRAINT IF EXISTS resource_config_scope_id_and_version_md5_unique;

DROP INDEX IF EXISTS resource_config_versions_check_order_idx;
DROP INDEX IF EXISTS resource_config_versions_version;

-- Rename Indexes
-- resource_disabled_versions
ALTER INDEX IF EXISTS resource_disabled_versions_resource_id_version_md5_uniq
RENAME TO resource_disabled_versions_resource_id_version_digest_uniq;

-- resource_caches
ALTER INDEX IF EXISTS resource_caches_resource_config_id_version_md5_params_hash_uniq
RENAME to resource_caches_rsc_config_id_version_digest_params_hash_uniq;

CREATE OR REPLACE FUNCTION jsonb_coalesce_empty(value jsonb)
RETURNS jsonb AS $$
  SELECT
      CASE WHEN jsonb_typeof($1) != 'object'
          THEN '{}'::jsonb
          ELSE $1
  END
$$ LANGUAGE sql;

-- Convert all rows to their new sha256 values
WITH json_string_cte AS (
    SELECT
        rcv.id,
        rcv.version_sha256 AS old_version_sha256,
        COALESCE(
            '{' || string_agg('"' || kv.key || '":"' || kv.value || '"', ',' ORDER BY kv.key) || '}',
            '{}'
        ) AS json_string
    FROM resource_config_versions rcv
    LEFT JOIN jsonb_each_text(jsonb_coalesce_empty(rcv.version::jsonb)) AS kv ON true
    GROUP BY rcv.id, rcv.version_sha256
),
hashed_json_string_cte AS (
    SELECT
        json_string_cte.id,
        json_string_cte.old_version_sha256,
        encode(digest(json_string_cte.json_string, 'sha256'), 'hex') AS new_version_sha256
    FROM json_string_cte
)
UPDATE resource_config_versions rcv
SET version_sha256 = hjs.new_version_sha256
FROM hashed_json_string_cte hjs
WHERE rcv.id = hjs.id;

--- Recreate indexes
-- resource_config_versions
ALTER TABLE resource_config_versions
ADD CONSTRAINT resource_config_scope_id_and_version_sha256_unique
UNIQUE (resource_config_scope_id, version_sha256);

-- ensure lookups using version_md5 are still fast
CREATE INDEX resource_config_scope_id_and_version_md5_idx ON resource_config_versions(resource_config_scope_id, version_md5);

CREATE INDEX resource_config_versions_check_order_idx
ON resource_config_versions (resource_config_scope_id, check_order DESC);

CREATE INDEX resource_config_versions_version
ON resource_config_versions
USING gin(version jsonb_path_ops)
WITH (FASTUPDATE = false);

-- New records will not have md5 digests
ALTER TABLE resource_config_versions
ALTER COLUMN version_md5 DROP NOT NULL;

-- With digests regenerated, all records should have sha256 digests
ALTER TABLE resource_config_versions
ALTER COLUMN version_sha256 SET NOT NULL;

ALTER TABLE resource_config_versions
DROP CONSTRAINT temporary_digest_not_null;
