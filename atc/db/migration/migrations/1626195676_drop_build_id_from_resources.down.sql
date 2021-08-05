ALTER TABLE resources ADD COLUMN build_id bigint REFERENCES builds (id) ON DELETE SET NULL;
CREATE INDEX resources_build_id_idx ON resources (build_id);
