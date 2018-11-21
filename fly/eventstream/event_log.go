package eventstream

import (
	"fmt"

	"github.com/concourse/concourse/atc"
)

type EventLog struct {
	Log			string
	Timestamp	int64
}

func NewEventLogFromError(format string, error error) EventLog {
	log := fmt.Sprintf(format, error)
	return EventLog{Log: log}
}

func NewEventLogFromStatus(format string, status atc.BuildStatus, timestamp int64) EventLog {
	log := fmt.Sprintf(format, status)
	return EventLog{Log: log, Timestamp: timestamp}
}

func NewEventLog(format string, log string, timestamp int64) EventLog {
	log = fmt.Sprintf(format, log)
	return EventLog{Log: log, Timestamp: timestamp}
}

func AdditionalFormatting(event EventLog, options RenderOptions) string {
	if options.ShowTimestamp {
		return Timestamped(event)
	}

	return event.Log
}