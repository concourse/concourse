BEGIN;

  CREATE TABLE wall (
      message text NOT NULL,
      expires_at timestamp with time zone
  );

COMMIT;