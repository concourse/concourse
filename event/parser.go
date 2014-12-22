package event

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/event/v1event"
	"github.com/concourse/atc/event/v2event"
)

type UnknownVersionError struct {
	Version atc.EventVersion
}

func (err UnknownVersionError) Error() string {
	return "unknown event version: " + string(err.Version)
}

func ParseEvent(version atc.EventVersion, typ atc.EventType, payload []byte) (atc.Event, error) {
	// TODO switch on major version instead
	switch version {
	case "1.0":
		return v1event.ParseEvent(typ, payload)
	case "2.0":
		return v2event.ParseEvent(typ, payload)
	}

	return nil, UnknownVersionError{version}
}
