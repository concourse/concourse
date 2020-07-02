package concourse

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
	"github.com/vito/go-sse/sse"
)

//go:generate counterfeiter . BuildEvents

type BuildEvents interface {
	Accept(visitor BuildEventsVisitor) error
	Close() error
}

type buildEvents struct {
	src *sse.EventSource
}

//go:generate counterfeiter . BuildEventsVisitor

type BuildEventsVisitor interface {
	VisitEvent(event atc.Event) error
}

func (b buildEvents) Accept(visitor BuildEventsVisitor) error {
	se, err := b.src.Next()
	if err != nil {
		return err
	}
	switch se.Name {
	case "event":
		var message event.Message
		err := json.Unmarshal(se.Data, &message)
		if err != nil {
			return err
		}

		return visitor.VisitEvent(message.Event)

	case "end":
		return io.EOF

	default:
		return fmt.Errorf("unknown event name: %s", se.Name)
	}
}

func (b buildEvents) Close() error {
	return b.src.Close()
}

func (client *client) BuildEvents(buildID string) (BuildEvents, error) {
	sseEvents, err := client.connection.ConnectToEventStream(internal.Request{
		RequestName: atc.BuildEvents,
		Params: rata.Params{
			"build_id": buildID,
		},
	})
	if err != nil {
		return buildEvents{}, err
	}

	return buildEvents{sseEvents}, nil
}
