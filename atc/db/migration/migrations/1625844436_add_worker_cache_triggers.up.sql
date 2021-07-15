CREATE OR REPLACE FUNCTION notify_trigger() RETURNS trigger AS $trigger$
DECLARE
  rec RECORD;
  payload TEXT;
  column_name TEXT;
  column_value TEXT;
  payload_items JSONB;
BEGIN
  CASE TG_OP
  WHEN 'INSERT', 'UPDATE' THEN
     rec := NEW;
  WHEN 'DELETE' THEN
     rec := OLD;
  ELSE
     RAISE EXCEPTION 'Unknown TG_OP: "%". Should not occur!', TG_OP;
  END CASE;

  -- Add each requested fields to a data payload.
  FOR i IN 1 .. (TG_NARGS - 1) LOOP
    column_name := TG_ARGV[i];
    EXECUTE format('SELECT $1.%I::TEXT', column_name)
    INTO column_value
    USING rec;
    payload_items := coalesce(payload_items,'{}')::jsonb || json_build_object(column_name,column_value)::jsonb;
  END LOOP;
  -- Build the payload
  payload := json_build_object(
    'operation',TG_OP,
    'data',payload_items
  );
  -- Notify the channel
  PERFORM pg_notify(TG_ARGV[0], payload);

  RETURN rec;
END;
$trigger$ LANGUAGE plpgsql;

CREATE TRIGGER workers_upsert_or_delete_trigger AFTER INSERT OR UPDATE OR DELETE ON workers
  FOR EACH ROW EXECUTE PROCEDURE notify_trigger(worker_events_channel, name);

CREATE TRIGGER containers_insert_or_delete_trigger AFTER INSERT OR DELETE ON containers
  FOR EACH ROW EXECUTE PROCEDURE notify_trigger(container_events_channel, id, worker_name, build_id);
