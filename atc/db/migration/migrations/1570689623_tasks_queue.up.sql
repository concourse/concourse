BEGIN;
  CREATE TABLE tasks_queue (
    "id" text NOT NULL PRIMARY KEY,
    "platform" text NOT NULL,
    "team_id" serial NOT NULL,
    "worker_tag" text NOT NULL,
    "insert_time" timestamp with time zone DEFAULT now() NOT NULL
  );
COMMIT;
