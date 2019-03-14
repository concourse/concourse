package tsa

import (
	"encoding/json"
	"io"
)

type EventType string

const (
	EventTypeRegistered  EventType = "registered"
	EventTypeHeartbeated EventType = "heartbeated"
)

type Event struct {
	Type EventType `json:"event"`
}

type EventWriter struct {
	enc *json.Encoder
}

func NewEventWriter(dest io.Writer) EventWriter {
	return EventWriter{
		enc: json.NewEncoder(dest),
	}
}

func (w EventWriter) Registered() error {
	return w.enc.Encode(Event{Type: EventTypeRegistered})
}

func (w EventWriter) Heartbeated() error {
	return w.enc.Encode(Event{Type: EventTypeHeartbeated})
}

type EventReader struct {
	dec *json.Decoder
}

func NewEventReader(src io.Reader) EventReader {
	return EventReader{
		dec: json.NewDecoder(src),
	}
}

func (r EventReader) Next() (Event, error) {
	var evt Event
	err := r.dec.Decode(&evt)
	if err != nil {
		return Event{}, err
	}

	return evt, nil
}
