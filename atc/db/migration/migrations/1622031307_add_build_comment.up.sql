CREATE TABLE build_comments (
    build_id INTEGER PRIMARY KEY,
    comment text
);

ALTER TABLE build_comments
  ADD CONSTRAINT worker_artifacts_build_fkey FOREIGN KEY (build_id) REFERENCES builds(id) ON DELETE CASCADE;
