BEGIN;
  ALTER TABLE resource_types
    DROP CONSTRAINT resource_types_resource_config_id_fkey,
    ADD CONSTRAINT resource_types_resource_config_id_fkey FOREIGN KEY (resource_config_id) REFERENCES resource_configs(id) ON DELETE RESTRICT;
COMMIT;
