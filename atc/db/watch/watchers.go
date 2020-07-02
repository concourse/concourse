package watch

import (
	"fmt"
	"strings"

	"github.com/concourse/concourse/atc/db"
)

var ErrDisabled = fmt.Errorf("watching is disabled")

const eventsChannel = "watch_events"

const createNotifyTriggerFunction = `
CREATE OR REPLACE FUNCTION notify_trigger() RETURNS trigger AS $trigger$
DECLARE
  rec RECORD;
  payload TEXT;
  column_name TEXT;
  column_value TEXT;
  payload_items JSONB;
BEGIN
  -- Set record row depending on operation
  CASE TG_OP
  WHEN 'INSERT', 'UPDATE' THEN
     rec := NEW;
  WHEN 'DELETE' THEN
     rec := OLD;
  ELSE
     RAISE EXCEPTION 'Unknown TG_OP: "%". Should not occur!', TG_OP;
  END CASE;
  
  -- Get required fields
  FOREACH column_name IN ARRAY TG_ARGV LOOP
    EXECUTE format('SELECT $1.%I::TEXT', column_name)
    INTO column_value
    USING rec;
    payload_items := coalesce(payload_items,'{}')::jsonb || json_build_object(column_name,column_value)::jsonb;
  END LOOP;

  -- Build the payload
  payload := json_build_object(
    'operation',TG_OP,
    'table',TG_TABLE_NAME,
    'data',payload_items
  );

  -- Notify the channel
  PERFORM pg_notify('` + eventsChannel + `', payload);
  
  RETURN rec;
END;
$trigger$ LANGUAGE plpgsql;
`

type Notification struct {
	Operation string            `json:"operation"`
	Table     string            `json:"table"`
	Data      map[string]string `json:"data"`
}

type EventType string

const (
	Put    EventType = "PUT"
	Delete EventType = "DELETE"
)

type watchTable struct {
	table string
	idCol string

	insert     bool
	update     bool
	updateCols []string
	delete     bool
}

func createWatchEventsTrigger(tx db.Tx, t watchTable) error {
	triggerName := t.table + `_notify`
	_, err := tx.Exec(`DROP TRIGGER IF EXISTS ` + triggerName + ` ON ` + t.table)
	if err != nil {
		return fmt.Errorf("drop trigger: %w", err)
	}
	var operations []string
	if t.insert {
		operations = append(operations, "INSERT")
	}
	if t.update {
		updateCond := "UPDATE"
		if len(t.updateCols) > 0 {
			updateCond += " OF " + strings.Join(t.updateCols, ", ")
		}
		operations = append(operations, updateCond)
	}
	if t.delete {
		operations = append(operations, "DELETE")
	}
	if len(operations) == 0 {
		panic("trigger " + triggerName + " must listen on at least one operation!")
	}
	condition := strings.Join(operations, " OR ")
	_, err = tx.Exec(
		`CREATE TRIGGER ` + triggerName + ` AFTER ` + condition + ` ON ` + t.table +
			` FOR EACH ROW EXECUTE PROCEDURE notify_trigger(` + t.idCol + `)`,
	)
	if err != nil {
		return fmt.Errorf("create trigger %s (%s): %w", triggerName, condition, err)
	}

	return nil
}
