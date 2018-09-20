BEGIN;
  UPDATE teams SET auth=json_build_object('owner', auth::json);
COMMIT;
