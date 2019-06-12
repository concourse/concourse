BEGIN;

  CREATE TABLE resource_config_versions (
      "id" serial NOT NULL PRIMARY KEY,
      "resource_config_id" integer NOT NULL REFERENCES resource_configs (id) ON DELETE CASCADE,
      "version" jsonb NOT NULL,
      "version_md5" text NOT NULL,
      "metadata" jsonb NOT NULL DEFAULT 'null',
      "check_order" integer NOT NULL DEFAULT 0
  );

  ALTER TABLE resource_config_versions
    ADD CONSTRAINT "resource_config_id_and_version_md5_unique" UNIQUE ("resource_config_id", "version_md5");

  CREATE TABLE resource_disabled_versions (
    "resource_id" integer NOT NULL REFERENCES resources (id) ON DELETE CASCADE,
    "version_md5" text NOT NULL
  );

  CREATE UNIQUE INDEX resource_disabled_versions_resource_id_version_md5_uniq
  ON resource_disabled_versions (resource_id, version_md5);

  INSERT INTO resource_disabled_versions (resource_id, version_md5)
  SELECT vr.resource_id, md5(vr.version)
  FROM versioned_resources vr
  WHERE NOT enabled
  ON CONFLICT DO NOTHING;

  ALTER TABLE resource_configs
    ADD COLUMN last_checked timestamp with time zone NOT NULL DEFAULT '1970-01-01 00:00:00',
    ADD COLUMN check_error text;

  ALTER TABLE resource_types
    ADD COLUMN check_error text;

  CREATE TABLE build_resource_config_version_inputs (
      "build_id" integer NOT NULL REFERENCES builds (id) ON DELETE CASCADE,
      "resource_id" integer NOT NULL REFERENCES resources (id) ON DELETE CASCADE,
      "version_md5" text NOT NULL,
      "name" text NOT NULL
  );

  CREATE UNIQUE INDEX build_resource_config_version_inputs_uniq
  ON build_resource_config_version_inputs (build_id, resource_id, version_md5, name);

  INSERT INTO build_resource_config_version_inputs (build_id, resource_id, version_md5, name)
  SELECT bi.build_id, vr.resource_id, md5(vr.version), bi.name
  FROM build_inputs bi, versioned_resources vr
  WHERE bi.versioned_resource_id = vr.id
  ON CONFLICT DO NOTHING;

  CREATE TABLE build_resource_config_version_outputs (
      "build_id" integer NOT NULL REFERENCES builds (id) ON DELETE CASCADE,
      "resource_id" integer NOT NULL REFERENCES resources (id) ON DELETE CASCADE,
      "version_md5" text NOT NULL,
      "name" text NOT NULL
  );

  CREATE UNIQUE INDEX build_resource_config_version_outputs_uniq
  ON build_resource_config_version_outputs (build_id, resource_id, version_md5, name);

  INSERT INTO build_resource_config_version_outputs (build_id, resource_id, version_md5, name)
  SELECT bo.build_id, vr.resource_id, md5(vr.version), r.name
  FROM build_outputs bo, versioned_resources vr, resources r
  WHERE bo.versioned_resource_id = vr.id AND vr.resource_id = r.id
  ON CONFLICT DO NOTHING;

  TRUNCATE TABLE next_build_inputs;

  ALTER TABLE next_build_inputs
    ADD COLUMN resource_config_version_id integer NOT NULL REFERENCES resource_config_versions (id) ON DELETE CASCADE,
    ADD COLUMN resource_id integer NOT NULL REFERENCES resources (id) ON DELETE CASCADE,
    DROP COLUMN version_id;

  CREATE INDEX next_build_inputs_resource_config_version_id ON next_build_inputs (resource_config_version_id);

  TRUNCATE TABLE independent_build_inputs;

  ALTER TABLE independent_build_inputs
    ADD COLUMN resource_config_version_id integer NOT NULL REFERENCES resource_config_versions (id) ON DELETE CASCADE,
    ADD COLUMN resource_id integer NOT NULL REFERENCES resources (id) ON DELETE CASCADE,
    DROP COLUMN version_id;

  CREATE INDEX independent_build_inputs_resource_config_version_id ON independent_build_inputs (resource_config_version_id);

  DROP INDEX resource_caches_resource_config_id_version_params_hash_key;

  ALTER TABLE resource_caches ALTER COLUMN version TYPE jsonb USING version::jsonb;

  CREATE UNIQUE INDEX resource_caches_resource_config_id_version_params_hash_uniq
  ON resource_caches (resource_config_id, md5(version::text), params_hash);

  ALTER TABLE worker_resource_config_check_sessions
    DROP COLUMN team_id;

  DELETE FROM worker_resource_config_check_sessions;

  CREATE UNIQUE INDEX worker_resource_config_check_sessions_uniq
  ON worker_resource_config_check_sessions (resource_config_check_session_id, worker_base_resource_type_id);

  ALTER TABLE resources
    DROP COLUMN last_checked,
    DROP CONSTRAINT resources_resource_config_id_fkey,
    ADD CONSTRAINT resources_resource_config_id_fkey FOREIGN KEY (resource_config_id) REFERENCES resource_configs(id) ON DELETE RESTRICT;

  ALTER TABLE resource_types
    DROP COLUMN last_checked,
    DROP COLUMN version;

COMMIT;
