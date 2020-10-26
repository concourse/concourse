BEGIN;
  -- restore id for any newly written events
  UPDATE build_events SET build_id_old = build_id WHERE build_id_old IS NULL;

  -- drop the indexes for the bigint column
  DROP INDEX build_events_build_id_idx;
  DROP INDEX build_events_build_id_event_id;

  -- drop the bigint column
  ALTER TABLE build_events DROP COLUMN build_id;

  -- rename everything back
  ALTER TABLE build_events RENAME COLUMN build_id_old TO build_id;
  ALTER INDEX build_events_build_id_old_idx RENAME TO build_events_build_id_idx;
  ALTER INDEX build_events_build_id_old_event_id  RENAME TO build_events_build_id_event_id;
COMMIT;
