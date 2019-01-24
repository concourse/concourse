BEGIN;

  CREATE TYPE container_state_old AS ENUM (
      'creating',
      'created',
      'destroying'
  );

  CREATE TYPE volume_state_old AS ENUM (
      'creating',
      'created',
      'destroying'
  );

  ALTER TABLE containers ADD COLUMN best_if_used_by timestamp with time zone;
COMMIT;
