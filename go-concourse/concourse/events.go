package concourse

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse/eventstream"
	"github.com/concourse/concourse/go-concourse/concourse/internal"
)

type Events interface {
	NextEvent() (atc.Event, error)
	Close() error
}

func (client *client) BuildEvents(buildID string) (Events, error) {
	sseEvents, err := client.connection.ConnectToEventStream(internal.Request{
		RequestName: atc.BuildEvents,
		Params: map[string]string{
			"build_id": buildID,
		},
	})
	if err != nil {
		return nil, err
	}

	return eventstream.NewSSEEventStream(sseEvents), nil
}
