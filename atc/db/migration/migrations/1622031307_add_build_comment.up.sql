CREATE TABLE build_comments (
    build_id INTEGER PRIMARY KEY,
    comment TEXT NOT NULL DEFAULT ''
);

ALTER TABLE build_comments
  ADD CONSTRAINT build_comments_build_id_fkey FOREIGN KEY (build_id) REFERENCES builds(id) ON DELETE CASCADE;
