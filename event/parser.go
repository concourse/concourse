package event

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/concourse/atc"
)

type eventTable map[atc.EventType]eventVersions
type eventVersions map[atc.EventVersion]eventParser
type eventParser func([]byte) (atc.Event, error)

var events = eventTable{}

func unmarshaler(e atc.Event) func([]byte) (atc.Event, error) {
	return func(payload []byte) (atc.Event, error) {
		val := reflect.New(reflect.TypeOf(e))
		err := json.Unmarshal(payload, val.Interface())
		return val.Elem().Interface().(atc.Event), err
	}
}

func registerEvent(e atc.Event) {
	versions, found := events[e.EventType()]
	if !found {
		versions = eventVersions{}
		events[e.EventType()] = versions
	}

	versions[e.Version()] = unmarshaler(e)
}

func init() {
	registerEvent(Input{})
	registerEvent(InputV10{})

	registerEvent(Output{})
	registerEvent(OutputV10{})

	registerEvent(Initialize{})

	registerEvent(Start{})

	registerEvent(Status{})

	registerEvent(Log{})

	registerEvent(Error{})

	registerEvent(Finish{})
}

func ParseEvent(version atc.EventVersion, typ atc.EventType, payload []byte) (atc.Event, error) {
	versions, found := events[typ]
	if !found {
		return nil, fmt.Errorf("unknown event type: %s", typ)
	}

	parser, found := versions[version]
	if !found {
		return nil, fmt.Errorf("unknown version of event: %s v%s", typ, version)
	}

	return parser(payload)
}
