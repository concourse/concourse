BEGIN;
  UPDATE teams SET auth = auth::json->'admin';
COMMIT;
