BEGIN;
  ALTER TABLE resources
    ADD COLUMN pin_comment text;

  UPDATE resources
  SET pin_comment = 'This resource is now pinned as a result of upgrading your Concourse. View the v5.0.0 release notes for more information.'
  WHERE api_pinned_version IS NOT NULL;
COMMIT;
