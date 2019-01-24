BEGIN;
  ALTER TABLE resources
    ADD COLUMN api_pinned_version jsonb,
    ADD COLUMN pin_comment text;

  UPDATE resources
  SET (api_pinned_version, pin_comment) =
    (resource_pins.version, resource_pins.comment_text)
  FROM resource_pins WHERE resources.id = resource_pins.resource_id;

  DROP TABLE resource_pins;
COMMIT;
