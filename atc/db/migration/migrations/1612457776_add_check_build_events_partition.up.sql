
CREATE TABLE check_build_events () INHERITS (build_events);
CREATE UNIQUE INDEX check_build_events_build_id_event_id ON check_build_events (build_id, event_id);

