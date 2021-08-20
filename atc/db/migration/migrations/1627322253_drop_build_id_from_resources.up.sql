DROP INDEX resources_build_id_idx;
ALTER TABLE resources
    DROP COLUMN build_id;
