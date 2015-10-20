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

func (handler AtcHandler) BuildEvents(buildID string) (Events, error) {
	sseEvents, err := handler.client.ConnectToEventStream(Request{
		RequestName: atc.BuildEvents,
		Params:      rata.Params{"build_id": buildID},
	})

	if err != nil {
		return nil, err
	}

	return eventstream.NewSSEEventStream(sseEvents), nil
}
