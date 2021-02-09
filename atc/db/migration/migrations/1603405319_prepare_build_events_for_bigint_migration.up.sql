BEGIN;
  -- rename old column and indexes
  ALTER TABLE build_events RENAME COLUMN build_id TO build_id_old;
  DROP INDEX build_events_build_id_idx;
  ALTER INDEX build_events_build_id_event_id RENAME TO build_events_build_id_old_event_id;

  -- add new bigint column which will be populated gradually at runtime
  ALTER TABLE build_events ADD COLUMN build_id bigint;

  -- create a replacement index along with what will become the new primary key
  CREATE UNIQUE INDEX build_events_build_id_event_id ON build_events (build_id, event_id);
COMMIT;
