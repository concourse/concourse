BEGIN;
    ALTER TABLE base_resource_types DROP COLUMN defaults;
COMMIT;