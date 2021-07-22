DROP INDEX IF EXISTS containers_image_check_container_id;
DROP INDEX IF EXISTS containers_image_get_container_id;

ALTER TABLE resource_cache_uses DROP COLUMN IF EXISTS image_check_container_id;
ALTER TABLE resource_cache_uses DROP COLUMN IF EXISTS image_get_container_id;
