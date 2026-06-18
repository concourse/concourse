ALTER TABLE workers ADD COLUMN stalled_since timestamp with time zone;

-- Backfill existing stalled workers so they are subject to the stall timeout
-- starting from the moment this migration runs, rather than being immortal due
-- to a NULL stalled_since.
UPDATE workers SET stalled_since = now() WHERE state = 'stalled';
