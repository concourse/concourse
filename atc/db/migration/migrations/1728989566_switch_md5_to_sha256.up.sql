CREATE EXTENSION IF NOT EXISTS pgcrypto;

ALTER TABLE resource_config_versions
RENAME COLUMN version_md5 TO version_sha256;

ALTER TABLE build_resource_config_version_inputs
RENAME COLUMN version_md5 TO version_sha256;

ALTER TABLE build_resource_config_version_outputs
RENAME COLUMN version_md5 TO version_sha256;

ALTER TABLE next_build_inputs
RENAME COLUMN version_md5 TO version_sha256;

ALTER TABLE resource_caches
RENAME COLUMN version_md5 TO version_sha256;

ALTER TABLE resource_disabled_versions
RENAME COLUMN version_md5 TO version_sha256;


-- CONSTRAINTs
ALTER TABLE resource_config_versions
  DROP CONSTRAINT IF EXISTS "resource_config_scope_id_and_version_md5_unique",
  ADD CONSTRAINT "resource_config_scope_id_and_version_sha256_unique" UNIQUE ("resource_config_scope_id", "version_sha256");


-- UNIQUE INDEXs
DROP INDEX IF EXISTS resource_disabled_versions_resource_id_version_md5_uniq;
CREATE UNIQUE INDEX resource_disabled_versions_resource_id_version_sha256_uniq
ON resource_disabled_versions (resource_id, version_sha256);

DROP INDEX IF EXISTS resource_caches_resource_config_id_version_md5_params_hash_uniq;
CREATE UNIQUE INDEX resource_caches_resource_config_id_version_sha256_params_hash_uniq
ON resource_caches (resource_config_id, version_sha256, params_hash);

DROP INDEX IF EXISTS build_inputs_resource_versions_idx;
CREATE INDEX build_inputs_resource_versions_idx ON build_resource_config_version_inputs (resource_id, version_sha256);

DROP INDEX IF EXISTS build_resource_config_version_inputs_uniq;
CREATE UNIQUE INDEX build_resource_config_version_inputs_uniq
ON build_resource_config_version_inputs (build_id, resource_id, version_sha256, name);

DROP INDEX IF EXISTS build_resource_config_version_outputs_uniq;
CREATE UNIQUE INDEX build_resource_config_version_outputs_uniq
ON build_resource_config_version_outputs (build_id, resource_id, version_sha256, name);
