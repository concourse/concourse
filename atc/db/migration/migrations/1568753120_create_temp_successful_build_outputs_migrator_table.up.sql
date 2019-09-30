BEGIN;

  CREATE TABLE successful_build_outputs_migrator (
    build_id_cursor int
  );

  INSERT INTO successful_build_outputs_migrator (build_id_cursor)
  SELECT id FROM builds ORDER BY id DESC LIMIT 1;

COMMIT;
