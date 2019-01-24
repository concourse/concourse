BEGIN;
  DELETE FROM "workers" WHERE (((state = 'stalled'::worker_state) OR (state = 'landed'::worker_state)) AND ((addr IS NOT NULL) OR (baggageclaim_url IS NOT NULL)));

  ALTER TABLE "workers"
  DROP CONSTRAINT IF EXISTS "addr_when_running",
  ADD CONSTRAINT "addr_when_running" CHECK ((((state <> 'stalled'::worker_state) AND (state <> 'landed'::worker_state) AND ((addr IS NOT NULL) OR (baggageclaim_url IS NOT NULL))) OR (((state = 'stalled'::worker_state) OR (state = 'landed'::worker_state)) AND (addr IS NULL) AND (baggageclaim_url IS NULL))));
COMMIT;
