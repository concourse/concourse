package v1event

import (
	"encoding/json"
	"fmt"

	"github.com/concourse/atc"
)

func ParseEvent(t atc.EventType, payload []byte) (atc.Event, error) {
	var ev atc.Event
	var err error

	switch t {
	case EventTypeLog:
		event := Log{}
		err = json.Unmarshal(payload, &event)
		ev = event
	case EventTypeStatus:
		event := Status{}
		err = json.Unmarshal(payload, &event)
		ev = event
	case EventTypeInitialize:
		event := Initialize{}
		err = json.Unmarshal(payload, &event)
		ev = event
	case EventTypeStart:
		event := Start{}
		err = json.Unmarshal(payload, &event)
		ev = event
	case EventTypeFinish:
		event := Finish{}
		err = json.Unmarshal(payload, &event)
		ev = event
	case EventTypeError:
		event := Error{}
		err = json.Unmarshal(payload, &event)
		ev = event
	case EventTypeInput:
		event := Input{}
		err = json.Unmarshal(payload, &event)
		ev = event
	case EventTypeOutput:
		event := Output{}
		err = json.Unmarshal(payload, &event)
		ev = event
	default:
		return nil, fmt.Errorf("unknown event type: %v", t)
	}

	return ev, err
}
