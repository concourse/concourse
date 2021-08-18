ALTER TABLE resource_cache_uses ADD COLUMN image_check_container_id integer REFERENCES containers(id) ON DELETE SET NULL;
ALTER TABLE resource_cache_uses ADD COLUMN image_get_container_id integer REFERENCES containers(id) ON DELETE SET NULL;

CREATE INDEX containers_image_check_container_id ON containers (image_check_container_id);
CREATE INDEX containers_image_get_container_id ON containers (image_get_container_id);
