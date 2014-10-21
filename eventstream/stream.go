package eventstream

import (
	"github.com/concourse/turbine/event"
	"github.com/vito/go-sse/sse"
)

type EventStream interface {
	NextEvent() (event.Event, error)
}

type SSEEventStream struct {
	sseReader *sse.Reader
}

func NewSSEEventStream(reader *sse.Reader) *SSEEventStream {
	return &SSEEventStream{sseReader: reader}
}

func (s *SSEEventStream) NextEvent() (event.Event, error) {
	se, err := s.sseReader.Next()
	if err != nil {
		return nil, err
	}

	return event.ParseEvent(event.EventType(se.Name), se.Data)
}
