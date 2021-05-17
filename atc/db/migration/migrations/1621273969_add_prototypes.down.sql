DROP INDEX builds_prototype_id_idx;
ALTER TABLE builds DROP COLUMN prototype_id;

DROP INDEX prototypes_resource_config_id;
DROP INDEX prototypes_pipeline_id;
DROP INDEX prototypes_pipeline_id_name_uniq;
DROP TABLE prototypes;
