BEGIN;
    ALTER TABLE base_resource_types ADD COLUMN defaults jsonb;
COMMIT;