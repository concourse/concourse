BEGIN;
  CREATE TABLE cache_invalidator (
      last_invalidated timestamp without time zone DEFAULT '1970-01-01 00:00:00'::timestamp without time zone NOT NULL
  );
COMMIT;
