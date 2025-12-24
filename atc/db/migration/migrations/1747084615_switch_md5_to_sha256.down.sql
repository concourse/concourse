--- Recreate indexes and constraint
ALTER TABLE resource_config_versions
DROP CONSTRAINT IF EXISTS resource_config_scope_id_and_version_sha256_unique;
-- Don't need to drop indexs resource_config_versions_check_order_idx, resource_config_versions_version because we're not modifying data

ALTER TABLE resource_config_versions
ADD CONSTRAINT resource_config_scope_id_and_version_md5_unique
UNIQUE (resource_config_scope_id, version_md5);

ALTER TABLE resource_config_versions
DROP COLUMN version_sha256;

ALTER TABLE resource_config_versions
ALTER COLUMN version_md5 SET NOT NULL;

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
