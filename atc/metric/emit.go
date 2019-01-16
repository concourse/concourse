package metric

import (
	"fmt"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"

	flags "github.com/jessevdk/go-flags"
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

const (
	EventStateOK       EventState = "ok"
	EventStateWarning  EventState = "warning"
	EventStateCritical EventState = "critical"
)

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

type eventEmission struct {
	event  Event
	logger lager.Logger
}

var (
	emitter         Emitter
	eventHost       string
	eventAttributes map[string]string
	emissions       chan eventEmission
)

func Initialize(logger lager.Logger, host string, attributes map[string]string) error {
	var (
		emitterDescriptions []string
		err                 error
	)

	for _, factory := range emitterFactories {
		if factory.IsConfigured() {
			emitterDescriptions = append(emitterDescriptions, factory.Description())
		}
	}
	if len(emitterDescriptions) > 1 {
		return fmt.Errorf("Multiple emitters configured: %s", strings.Join(emitterDescriptions, ", "))
	}

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
	emissions = make(chan eventEmission, 1000)

	go emitLoop()

	return nil
}

func Deinitialize(logger lager.Logger) {
	close(emissions)
	emitterFactories = nil
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
