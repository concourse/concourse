BEGIN;

  CREATE TABLE job_tags (
    job_id int REFERENCES jobs (id) ON DELETE CASCADE,
    tag text NOT NULL,
    UNIQUE (job_id, tag)
  );

  CREATE INDEX job_tags_job_id ON job_tags USING btree (job_id);

COMMIT;
