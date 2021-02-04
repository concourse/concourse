BEGIN;
  CREATE TABLE check_build_events () INHERITS (build_events);
  CREATE INDEX check_build_events_build_id ON check_build_events (build_id);
  CREATE INDEX check_build_events_build_id_event_id ON check_build_events (build_id, event_id);
COMMIT;
