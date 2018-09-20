BEGIN;
  UPDATE teams SET auth = auth::json->'owner';
COMMIT;
