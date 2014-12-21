package v2event

import (
	"encoding/json"
	"fmt"

	"github.com/concourse/atc"
)

func ParseEvent(t atc.EventType, payload []byte) (atc.Event, error) {
	var ev atc.Event
	var err error

	switch t {
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
