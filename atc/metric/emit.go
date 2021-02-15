package metric

import (
	"time"

	"code.cloudfoundry.org/lager"
)

type Event struct {
	Name       string
	Value      float64
	Attributes map[string]string
	Host       string
	Time       time.Time
}

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

func Initialize(logger lager.Logger, factory EmitterFactory, host string, attributes map[string]string, bufferSize uint32) error {
	logger.Debug("metric-initialize", lager.Data{
		"host":        host,
		"attributes":  attributes,
		"buffer-size": bufferSize,
	})

	var (
		err error
	)

	emitter, err = factory.NewEmitter()
	if err != nil {
		return err
	}

	if emitter == nil {
		return nil
	}

	eventHost = host
	eventAttributes = attributes
	emissions = make(chan eventEmission, int(bufferSize))

	go emitLoop()

	return nil
}

func Deinitialize(logger lager.Logger) {
	close(emissions)
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
