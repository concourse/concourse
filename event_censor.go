package auth

import (
	"encoding/json"
	"errors"

	"github.com/concourse/turbine/event"
	"github.com/vito/go-sse/sse"
)

var ErrVersionUnknown = errors.New("event stream version unknown")

type EventCensor struct {
	version string
}

func (c *EventCensor) Censor(e sse.Event) (sse.Event, error) {
	if e.Name == "version" {
		c.version = string(e.Data)
	}

	switch c.version {
	case "0.0":
		return e, nil

	case "1.0":
		switch event.EventType(e.Name) {
		case event.EventTypeInitialize:
			var te event.Initialize
			err := json.Unmarshal(e.Data, &te)
			if err != nil {
				return sse.Event{}, err
			}

			te.BuildConfig.Params = nil

			e.Data, err = json.Marshal(te)
			if err != nil {
				return sse.Event{}, err
			}

			return e, nil

		case event.EventTypeInput:
			var te event.Input
			err := json.Unmarshal(e.Data, &te)
			if err != nil {
				return sse.Event{}, err
			}

			te.Input.Source = nil
			te.Input.Params = nil

			e.Data, err = json.Marshal(te)
			if err != nil {
				return sse.Event{}, err
			}

			return e, nil

		case event.EventTypeOutput:
			var te event.Output
			err := json.Unmarshal(e.Data, &te)
			if err != nil {
				return sse.Event{}, err
			}

			te.Output.Source = nil
			te.Output.Params = nil

			e.Data, err = json.Marshal(te)
			if err != nil {
				return sse.Event{}, err
			}

			return e, nil

		}

		return e, nil

	default:
		return sse.Event{}, ErrVersionUnknown
	}
}
