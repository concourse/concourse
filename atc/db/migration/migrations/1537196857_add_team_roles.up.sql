BEGIN;
  UPDATE teams SET auth=json_build_object('admin', auth::json);
COMMIT;
