BEGIN;

  CREATE TABLE resource_config_versions (
      "id" serial NOT NULL,
      "resource_config_id" integer NOT NULL,
      "version" text NOT NULL,
      "metadata" text NOT NULL,
      "check_order" integer DEFAULT 0 NOT NULL
  );

  ALTER TABLE resource_config_versions
    ADD CONSTRAINT "resource_config_id_and_version_unique" UNIQUE ("resource_config_id", "version");

  CREATE TABLE pipeline_disabled_resource_config_versions (
    pipeline_id integer NOT NULL,
    resource_config_version_id integer NOT NULL
  );

  ALTER TABLE resource_configs
    ADD COLUMN last_checked timestamp with time zone DEFAULT now()
    NOT NULL;

  ALTER TABLE resource_configs
    ALTER COLUMN last_checked
    SET DEFAULT '1970-01-01 00:00:00'::timestamp with time zone;

COMMIT;
