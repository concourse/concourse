BEGIN;

  CREATE TABLE resource_config_versions (
      "id" serial NOT NULL PRIMARY KEY,
      "resource_config_id" integer NOT NULL REFERENCES resource_configs (id) ON DELETE CASCADE,
      "version" jsonb NOT NULL,
      "metadata" jsonb NOT NULL DEFAULT 'null',
      "check_order" integer NOT NULL DEFAULT 0
  );

  ALTER TABLE resource_config_versions
    ADD CONSTRAINT "resource_config_id_and_version_unique" UNIQUE ("resource_config_id", "version");

  CREATE TABLE resource_disabled_versions (
    "resource_id" integer NOT NULL REFERENCES resources (id) ON DELETE CASCADE,
    "version" jsonb NOT NULL
  );

  INSERT INTO resource_disabled_versions (resource_id, version)
  SELECT vr.resource_id, vr.version::jsonb
  FROM versioned_resources vr
  WHERE NOT enabled;

  ALTER TABLE resource_configs
    ADD COLUMN last_checked timestamp NOT NULL DEFAULT '1970-01-01 00:00:00'::timestamp with time zone,
    ADD COLUMN check_error text;

  ALTER TABLE resource_types
    ADD COLUMN check_error text;

  CREATE TABLE build_resource_config_versions_inputs (
      "build_id" integer NOT NULL REFERENCES builds (id) ON DELETE CASCADE,
      "resource_config_version_id" integer NOT NULL REFERENCES resource_config_versions (id) ON DELETE CASCADE,
      "name" text NOT NULL
  );

  CREATE TABLE build_resource_config_versions_outputs (
      "build_id" integer NOT NULL REFERENCES builds (id) ON DELETE CASCADE,
      "resource_config_version_id" integer NOT NULL REFERENCES resource_config_versions (id) ON DELETE CASCADE,
      "name" text NOT NULL
  );

  TRUNCATE TABLE next_build_inputs;

  ALTER TABLE next_build_inputs
    ADD COLUMN resource_config_version_id integer NOT NULL REFERENCES resource_config_versions (id) ON DELETE CASCADE,
    DROP COLUMN version_id;

  TRUNCATE TABLE independent_build_inputs;

  ALTER TABLE independent_build_inputs
    ADD COLUMN resource_config_version_id integer NOT NULL REFERENCES resource_config_versions (id) ON DELETE CASCADE,
    DROP COLUMN version_id;

  DROP INDEX resource_caches_resource_config_id_version_params_hash_key;

  ALTER TABLE resource_caches ALTER COLUMN version TYPE jsonb USING version::jsonb;

  CREATE UNIQUE INDEX resource_caches_resource_config_id_version_params_hash_uniq
  ON resource_caches (resource_config_id, md5(version::text), params_hash);

COMMIT;
