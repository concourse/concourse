ALTER TABLE resource_cache_uses ADD COLUMN container_id integer REFERENCES containers(id) ON DELETE CASCADE;
CREATE INDEX resource_cache_uses_container_id ON resource_cache_uses(container_id);
