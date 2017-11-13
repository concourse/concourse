BEGIN;

ALTER INDEX versioned_resources_resource_space_id_idx RENAME TO versioned_resources_resource_id_idx;

ALTER INDEX versioned_resources_resource_space_id_type_version RENAME TO versioned_resources_resource_id_type_version;

ALTER TABLE versioned_resources RENAME resource_space_id TO resource_id;

ALTER TABLE versioned_resources DROP CONSTRAINT resource_space_id_fkey;

ALTER TABLE versioned_resources ADD CONSTRAINT fkey_resource_id FOREIGN KEY (resource_id) REFERENCES resources(id) ON DELETE CASCADE;

DROP TABLE resource_spaces;

COMMIT;
