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

type BuildEvents struct {
	src *sse.EventSource
}

func (b BuildEvents) NextEvent() (atc.Event, error) {
	se, err := b.src.Next()
	if err != nil {
		return nil, err
	}
	switch se.Name {
	case "event":
		var message event.Message
		err := json.Unmarshal(se.Data, &message)
		if err != nil {
			return nil, err
		}

		return message.Event, nil

	case "end":
		return nil, io.EOF

	default:
		return nil, fmt.Errorf("unknown event name: %s", se.Name)
	}
}

func (b BuildEvents) Close() error {
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
		return BuildEvents{}, err
	}

	return BuildEvents{sseEvents}, nil
}
