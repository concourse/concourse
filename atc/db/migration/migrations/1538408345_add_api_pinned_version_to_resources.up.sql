BEGIN;
  ALTER TABLE resources
    ADD COLUMN api_pinned_version jsonb;

  WITH latest_resource_config_versions AS (
    SELECT DISTINCT ON (resource_config_id)
      resource_config_id, version
    FROM resource_config_versions
    ORDER BY resource_config_id, check_order DESC
  ) UPDATE resources r
    SET api_pinned_version = v.version
    FROM latest_resource_config_versions v
    WHERE r.resource_config_id = v.resource_config_id
    AND r.paused = TRUE;

COMMIT;
