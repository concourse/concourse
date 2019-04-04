package event

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/concourse/concourse/atc"
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

func RegisterEvent(e atc.Event) {
	versions, found := events[e.EventType()]
	if !found {
		versions = eventVersions{}
		events[e.EventType()] = versions
	}

	versions[e.Version()] = unmarshaler(e)
}

func init() {
	RegisterEvent(InitializeTask{})
	RegisterEvent(StartTask{})
	RegisterEvent(FinishTask{})
	RegisterEvent(InitializeGet{})
	RegisterEvent(StartGet{})
	RegisterEvent(FinishGet{})
	RegisterEvent(InitializePut{})
	RegisterEvent(StartPut{})
	RegisterEvent(FinishPut{})
	RegisterEvent(Status{})
	RegisterEvent(Log{})
	RegisterEvent(Error{})

	// deprecated:
	RegisterEvent(InitializeV10{})
	RegisterEvent(FinishV10{})
	RegisterEvent(StartV10{})
	RegisterEvent(InputV10{})
	RegisterEvent(InputV20{})
	RegisterEvent(OutputV10{})
	RegisterEvent(OutputV20{})
	RegisterEvent(ErrorV10{})
	RegisterEvent(ErrorV20{})
	RegisterEvent(ErrorV30{})
	RegisterEvent(FinishTaskV10{})
	RegisterEvent(FinishTaskV20{})
	RegisterEvent(FinishTaskV30{})
	RegisterEvent(InitializeTaskV10{})
	RegisterEvent(InitializeTaskV20{})
	RegisterEvent(InitializeTaskV30{})
	RegisterEvent(StartTaskV10{})
	RegisterEvent(StartTaskV20{})
	RegisterEvent(StartTaskV30{})
	RegisterEvent(StartTaskV40{})
	RegisterEvent(LogV10{})
	RegisterEvent(LogV20{})
	RegisterEvent(LogV30{})
	RegisterEvent(LogV40{})
	RegisterEvent(FinishGetV10{})
	RegisterEvent(FinishGetV20{})
	RegisterEvent(FinishGetV30{})
	RegisterEvent(FinishPutV10{})
	RegisterEvent(FinishPutV20{})
	RegisterEvent(FinishPutV30{})
	RegisterEvent(InitializeGetV10{})
	RegisterEvent(InitializePutV10{})
	RegisterEvent(FinishGetV40{})
	RegisterEvent(FinishPutV40{})
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

type UnknownEventTypeError struct {
	Type atc.EventType
}

func (err UnknownEventTypeError) Error() string {
	return fmt.Sprintf("unknown event type: %s", err.Type)
}

type UnknownEventVersionError struct {
	Type          atc.EventType
	Version       atc.EventVersion
	KnownVersions []string
}

func (err UnknownEventVersionError) Error() string {
	return fmt.Sprintf(
		"unknown event version: %s version %s (supported versions: %s)",
		err.Type,
		err.Version,
		strings.Join(err.KnownVersions, ", "),
	)
}

func ParseEvent(version atc.EventVersion, typ atc.EventType, payload []byte) (atc.Event, error) {
	versions, found := events[typ]
	if !found {
		return nil, UnknownEventTypeError{typ}
	}

	knownVersions := []string{}
	for v, parser := range versions {
		knownVersions = append(knownVersions, string(v))

		if v.IsCompatibleWith(version) {
			return parser(payload)
		}
	}

	return nil, UnknownEventVersionError{
		Type:          typ,
		Version:       version,
		KnownVersions: knownVersions,
	}
}
