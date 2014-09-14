package logfanout

import (
	"encoding/json"
	"errors"

	"github.com/concourse/turbine/event"
)

var ErrVersionUnknown = errors.New("event stream version unknown")

type CensoredSink struct {
	conn JSONWriteCloser

	version string
}

func NewCensoredSink(conn JSONWriteCloser) Sink {
	return &CensoredSink{
		conn: conn,
	}
}

func (sink *CensoredSink) WriteMessage(msg *json.RawMessage) error {
	// always try to parse a version out of the message
	var version VersionMessage
	err := json.Unmarshal([]byte(*msg), &version)
	if err != nil {
		return err
	}

	if version.Version != "" {
		sink.version = version.Version
		return sink.conn.WriteJSON(msg)
	}

	switch sink.version {
	case "0.0":
		return sink.conn.WriteJSON(msg)

	case "1.0":
		var message event.Message
		err := json.Unmarshal([]byte(*msg), &message)
		if err != nil {
			return err
		}

		switch ev := message.Event.(type) {
		case event.Initialize:
			ev.BuildConfig.Params = nil

			message.Event = ev

		case event.Input:
			ev.Input.Source = nil
			ev.Input.Params = nil

			message.Event = ev

		case event.Output:
			ev.Output.Source = nil
			ev.Output.Params = nil

			message.Event = ev
		}

		return sink.conn.WriteJSON(message)

	default:
		return ErrVersionUnknown
	}
}

func (sink *CensoredSink) Close() error {
	return sink.conn.Close()
}
