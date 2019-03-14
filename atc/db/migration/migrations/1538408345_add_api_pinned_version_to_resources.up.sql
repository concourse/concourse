BEGIN;
  ALTER TABLE resources
    ADD COLUMN api_pinned_version jsonb;

  WITH latest_resource_versions AS (
    SELECT DISTINCT ON (resource_id)
      resource_id, version
    FROM versioned_resources
    ORDER BY resource_id, check_order DESC
  ) UPDATE resources r
    SET api_pinned_version = v.version::jsonb
    FROM latest_resource_versions v
    WHERE r.id = v.resource_id
    AND r.paused = TRUE;
COMMIT;
