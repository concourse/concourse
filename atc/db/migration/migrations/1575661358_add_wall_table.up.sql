BEGIN;

  CREATE TABLE wall (
      message text NOT NULL,
      expires timestamp with time zone
  );

COMMIT;