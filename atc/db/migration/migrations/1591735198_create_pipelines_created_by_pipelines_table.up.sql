BEGIN;
    CREATE TABLE pipelines_created_by_pipelines (
        pipeline_id integer REFERENCES pipelines(id) NOT NULL,
        job_id integer REFERENCES jobs(id) NOT NULL,
        build_id integer REFERENCES builds(id) NOT NULL
    );

COMMIT;
