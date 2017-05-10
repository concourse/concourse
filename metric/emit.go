package metric

import (
	"time"

	"code.cloudfoundry.org/lager"
)

type Event struct {
	Name       string
	Value      interface{}
	State      EventState
	Attributes map[string]string

	Host string
	Time int64
	Tags []string
}

type EventState string

const EventStateOK EventState = "ok"
const EventStateWarning EventState = "warning"
const EventStateCritical EventState = "critical"

type Emitter interface {
	Emit(lager.Logger, Event)
}

var emitter Emitter
var eventHost string
var eventTags []string
var eventAttributes map[string]string

type eventEmission struct {
	event  Event
	logger lager.Logger
}

var emissions = make(chan eventEmission, 1000)

func Initialize(logger lager.Logger, emitter Emitter, host string, tags []string, attributes map[string]string) {
	emitter = emitter
	eventHost = host
	eventTags = tags
	eventAttributes = attributes

	go emitLoop()
}

func emit(logger lager.Logger, event Event) {
	logger.Debug("emit")

	if emitter == nil {
		return
	}

	event.Host = eventHost
	event.Time = time.Now().Unix()
	event.Tags = append(event.Tags, eventTags...)

	mergedAttributes := map[string]string{}
	for k, v := range eventAttributes {
		mergedAttributes[k] = v
	}

	if event.Attributes != nil {
		for k, v := range event.Attributes {
			mergedAttributes[k] = v
		}
	}

	event.Attributes = mergedAttributes

	select {
	case emissions <- eventEmission{logger: logger, event: event}:
	default:
		logger.Error("queue-full", nil)
	}
}

func emitLoop() {
	for emission := range emissions {
		emitter.Emit(emission.logger, emission.event)
	}
}
