package atcclient

import (
	"github.com/concourse/atc"
	"github.com/concourse/fly/atcclient/eventstream"
	"github.com/tedsuo/rata"
)

type Events interface {
	NextEvent() (atc.Event, error)
	Close() error
}

func (client *client) BuildEvents(buildID string) (Events, error) {
	sseEvents, err := client.connection.ConnectToEventStream(Request{
		RequestName: atc.BuildEvents,
		Params:      rata.Params{"build_id": buildID},
	})

	if err != nil {
		return nil, err
	}

	return eventstream.NewSSEEventStream(sseEvents), nil
}
