package metric

import (
	"fmt"
	"strings"
	"time"

	flags "github.com/jessevdk/go-flags"

	"code.cloudfoundry.org/lager"
)

type Event struct {
	Name       string
	Value      interface{}
	State      EventState
	Attributes map[string]string
	Host       string
	Time       time.Time
}

type EventState string

const EventStateOK EventState = "ok"
const EventStateWarning EventState = "warning"
const EventStateCritical EventState = "critical"

//go:generate counterfeiter . Emitter
type Emitter interface {
	Emit(lager.Logger, Event)
}

//go:generate counterfeiter . EmitterFactory
type EmitterFactory interface {
	Description() string
	IsConfigured() bool
	NewEmitter() (Emitter, error)
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
var eventAttributes map[string]string

type eventEmission struct {
	event  Event
	logger lager.Logger
}

var emissions = make(chan eventEmission, 1000)

func Initialize(logger lager.Logger, host string, attributes map[string]string) error {
	var emitterDescriptions []string
	for _, factory := range emitterFactories {
		if factory.IsConfigured() {
			emitterDescriptions = append(emitterDescriptions, factory.Description())
		}
	}
	if len(emitterDescriptions) > 1 {
		return fmt.Errorf("Multiple emitters configured: %s", strings.Join(emitterDescriptions, ", "))
	}

	var err error
	for _, factory := range emitterFactories {
		if factory.IsConfigured() {
			emitter, err = factory.NewEmitter()
			if err != nil {
				return err
			}
		}
	}

	if emitter == nil {
		return nil
	}

	emitter = emitter
	eventHost = host
	eventAttributes = attributes

	go emitLoop()

	return nil
}

func emit(logger lager.Logger, event Event) {
	if emitter == nil {
		return
	}

	event.Host = eventHost
	event.Time = time.Now()

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
