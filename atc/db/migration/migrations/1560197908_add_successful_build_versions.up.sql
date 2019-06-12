BEGIN;
  CREATE TABLE successful_build_versions (
      "build_id" integer NOT NULL REFERENCES builds (id) ON DELETE CASCADE,
      "resource_id" integer NOT NULL REFERENCES resources (id) ON DELETE CASCADE,
      "version_md5" text NOT NULL,
      "job_id" integer NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
      "name" text NOT NULL
  );

  CREATE INDEX successful_build_versions_build_id_idx ON successful_build_versions (build_id);

  CREATE UNIQUE INDEX successful_build_versions_unique_idx ON successful_build_versions (resource_id, version_md5, job_id, build_id, name);

  INSERT INTO successful_build_versions (build_id, resource_id, version_md5, job_id, name)
  SELECT i.build_id, i.resource_id, i.version_md5, b.job_id, i.name
  FROM build_resource_config_version_inputs i
  JOIN builds b ON b.id = i.build_id
  WHERE b.status = 'succeeded'
  ON CONFLICT DO NOTHING;

  INSERT INTO successful_build_versions (build_id, resource_id, version_md5, job_id, name)
  SELECT o.build_id, o.resource_id, o.version_md5, b.job_id, o.name
  FROM build_resource_config_version_outputs o
  JOIN builds b ON b.id = o.build_id
  WHERE b.status = 'succeeded'
  ON CONFLICT DO NOTHING;

COMMIT;
