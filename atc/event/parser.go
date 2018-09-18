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
	registerEvent(InitializeTask{})
	registerEvent(StartTask{})
	registerEvent(FinishTask{})
	registerEvent(FinishGet{})
	registerEvent(FinishPut{})
	registerEvent(Status{})
	registerEvent(Log{})
	registerEvent(Error{})

	// deprecated:
	registerEvent(InitializeV10{})
	registerEvent(FinishV10{})
	registerEvent(StartV10{})
	registerEvent(InputV10{})
	registerEvent(InputV20{})
	registerEvent(OutputV10{})
	registerEvent(OutputV20{})
	registerEvent(ErrorV10{})
	registerEvent(ErrorV20{})
	registerEvent(ErrorV30{})
	registerEvent(FinishTaskV10{})
	registerEvent(FinishTaskV20{})
	registerEvent(FinishTaskV30{})
	registerEvent(InitializeTaskV10{})
	registerEvent(InitializeTaskV20{})
	registerEvent(InitializeTaskV30{})
	registerEvent(StartTaskV10{})
	registerEvent(StartTaskV20{})
	registerEvent(StartTaskV30{})
	registerEvent(StartTaskV40{})
	registerEvent(LogV10{})
	registerEvent(LogV20{})
	registerEvent(LogV30{})
	registerEvent(LogV40{})
	registerEvent(LogV50{})
	registerEvent(FinishGetV10{})
	registerEvent(FinishGetV20{})
	registerEvent(FinishGetV30{})
	registerEvent(FinishPutV10{})
	registerEvent(FinishPutV20{})
	registerEvent(FinishPutV30{})
	registerEvent(InitializeGetV10{})
	registerEvent(InitializePutV10{})
	registerEvent(FinishGetV40{})
	registerEvent(FinishPutV40{})
}

type Message struct {
	Event atc.Event
}

type Envelope struct {
	Data    *json.RawMessage `json:"data"`
	Event   atc.EventType    `json:"event"`
	Version atc.EventVersion `json:"version"`
}

func (m Message) MarshalJSON() ([]byte, error) {
	var envelope Envelope

	payload, err := json.Marshal(m.Event)
	if err != nil {
		return nil, err
	}

	envelope.Data = (*json.RawMessage)(&payload)
	envelope.Event = m.Event.EventType()
	envelope.Version = m.Event.Version()

	return json.Marshal(envelope)
}

func (m *Message) UnmarshalJSON(bytes []byte) error {
	var envelope Envelope

	err := json.Unmarshal(bytes, &envelope)
	if err != nil {
		return err
	}

	event, err := ParseEvent(envelope.Version, envelope.Event, *envelope.Data)
	if err != nil {
		return err
	}

	m.Event = event

	return nil
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
