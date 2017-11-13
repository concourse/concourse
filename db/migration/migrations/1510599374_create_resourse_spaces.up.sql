BEGIN;

CREATE TABLE resource_spaces (
  id serial PRIMARY KEY,
  resource_id int REFERENCES resources (id) ON DELETE CASCADE,
  name text NOT NULL,
  UNIQUE (resource_id, name)
);

INSERT INTO resource_spaces(id, resource_id, name) SELECT id, id, 'default' from resources;

ALTER TABLE versioned_resources RENAME resource_id TO resource_space_id;

ALTER TABLE versioned_resources DROP CONSTRAINT fkey_resource_id;

ALTER TABLE versioned_resources ADD CONSTRAINT resource_space_id_fkey FOREIGN KEY (resource_space_id) REFERENCES resource_spaces (id) ON DELETE CASCADE;

ALTER INDEX versioned_resources_resource_id_type_version RENAME TO versioned_resources_resource_space_id_type_version;

ALTER INDEX versioned_resources_resource_id_idx RENAME TO versioned_resources_resource_space_id_idx;

COMMIT;
