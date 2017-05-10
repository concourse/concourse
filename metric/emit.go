package metric

import (
	"fmt"
	"time"

	flags "github.com/jessevdk/go-flags"

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

type EmitterFactory interface {
	Description() string
	IsConfigured() bool
	NewEmitter() Emitter
}

var emitterFactories []EmitterFactory

func RegisterEmitter(factory EmitterFactory) {
	emitterFactories = append(emitterFactories, factory)
}

func WireEmitters(group *flags.Group) {
	for _, factory := range emitterFactories {
		_, err := group.AddGroup(fmt.Sprintf("Metric Emitter (%s)", factory.Description()), "", factory)
		if err != nil {
			panic(err)
		}
	}
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

func Initialize(logger lager.Logger, host string, tags []string, attributes map[string]string) {
	for _, factory := range emitterFactories {
		if factory.IsConfigured() {
			emitter = factory.NewEmitter()
		}
	}

	if emitter == nil {
		return
	}

	emitter = emitter
	eventHost = host
	eventTags = tags
	eventAttributes = attributes

	go emitLoop()
}

func emit(logger lager.Logger, event Event) {
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
		emitter.Emit(emission.logger.Session("emit"), emission.event)
	}
}
