-- Calculate md5 digest for any new versions. These will be rows where version_md5 is null
WITH json_string_cte AS (
    SELECT
        rcv.id,
        rcv.version_sha256,
        COALESCE(
            '{' || string_agg('"' || kv.key || '":"' || kv.value || '"', ',' ORDER BY kv.key) || '}',
            '{}'
        ) AS json_string
    FROM resource_config_versions rcv
    LEFT JOIN jsonb_each_text(rcv.version::jsonb) AS kv ON true
    WHERE jsonb_typeof(rcv.version::jsonb) = 'object' AND rcv.version_md5 IS NULL
    GROUP BY rcv.id, rcv.version_sha256
),
hashed_json_string_cte AS (
    SELECT
        json_string_cte.id,
        json_string_cte.version_sha256,
        md5(json_string_cte.json_string) AS version_md5
    FROM json_string_cte
),
update_resource_versions AS (
    UPDATE resource_config_versions rcv
    SET version_md5 = hjs.version_md5
    FROM hashed_json_string_cte hjs
    WHERE rcv.id = hjs.id
),
update_resource_disabled_versions AS (
    UPDATE resource_disabled_versions rdv
    SET version_digest = hjs.version_md5
    FROM hashed_json_string_cte hjs
    WHERE rdv.version_digest = hjs.version_sha256
),
update_build_resource_config_version_inputs AS (
    UPDATE build_resource_config_version_inputs bri
    SET version_digest = hjs.version_md5
    FROM hashed_json_string_cte hjs
    WHERE bri.version_digest = hjs.version_sha256
),
update_build_resource_config_version_outputs AS (
    UPDATE build_resource_config_version_outputs bro
    SET version_digest = hjs.version_md5
    FROM hashed_json_string_cte hjs
    WHERE bro.version_digest = hjs.version_sha256
),
update_resource_caches AS (
    UPDATE resource_caches rc
    SET version_digest = hjs.version_md5
    FROM hashed_json_string_cte hjs
    WHERE rc.version_digest = hjs.version_sha256
)
UPDATE next_build_inputs nbi
SET version_digest = hjs.version_md5
FROM hashed_json_string_cte hjs
WHERE nbi.version_digest = hjs.version_sha256;

--- Recreate indexes and constraint
ALTER TABLE resource_config_versions
DROP CONSTRAINT IF EXISTS resource_config_scope_id_and_version_sha256_unique;
-- Don't need to drop indexes resource_config_versions_check_order_idx, resource_config_versions_version because we're not modifying data

ALTER TABLE resource_config_versions
ADD CONSTRAINT resource_config_scope_id_and_version_md5_unique
UNIQUE (resource_config_scope_id, version_md5);

ALTER TABLE resource_config_versions
DROP COLUMN version_sha256;

ALTER TABLE resource_config_versions
ALTER COLUMN version_md5 SET NOT NULL;

DROP INDEX resource_config_scope_id_and_version_md5_idx;
DROP INDEX resource_config_versions_version_md5;

-- Revert column renames from version_digest back to version_md5
ALTER TABLE build_resource_config_version_inputs
RENAME COLUMN version_digest TO version_md5;

ALTER TABLE build_resource_config_version_outputs
RENAME COLUMN version_digest TO version_md5;

ALTER TABLE next_build_inputs
RENAME COLUMN version_digest TO version_md5;

ALTER TABLE resource_caches
RENAME COLUMN version_digest TO version_md5;

ALTER TABLE resource_disabled_versions
RENAME COLUMN version_digest TO version_md5;

-- Rename Indexes
-- resource_disabled_versions
ALTER INDEX IF EXISTS resource_disabled_versions_resource_id_version_digest_uniq
RENAME TO resource_disabled_versions_resource_id_version_md5_uniq;

-- resource_caches
ALTER INDEX IF EXISTS resource_caches_resource_config_id_version_digest_params_hash_uniq
RENAME to resource_caches_resource_config_id_version_md5_params_hash_uniq;
