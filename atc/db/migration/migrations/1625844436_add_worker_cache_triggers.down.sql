DROP TRIGGER IF EXISTS workers_upsert_or_delete_trigger ON workers;
DROP TRIGGER IF EXISTS containers_insert_or_delete_trigger ON containers;

DROP FUNCTION IF EXISTS notify_trigger;
