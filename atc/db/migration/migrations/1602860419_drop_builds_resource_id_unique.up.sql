
ALTER TABLE builds DROP CONSTRAINT builds_resource_id_key;
CREATE INDEX builds_resource_id_idx ON builds (resource_id);

ALTER TABLE resources ADD COLUMN build_id bigint REFERENCES builds (id) ON DELETE SET NULL;

