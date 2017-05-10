package emitter

import (
	"code.cloudfoundry.org/lager"
	"github.com/The-Cloud-Source/goryman"
	"github.com/concourse/atc/metric"
)

type RiemannEmitter struct {
	client    *goryman.GorymanClient
	connected bool

	servicePrefix string
}

func NewRiemannEmitter(addr string, servicePrefix string) metric.Emitter {
	return &RiemannEmitter{
		client:    goryman.NewGorymanClient(addr),
		connected: false,

		servicePrefix: servicePrefix,
	}
}

func (emitter *RiemannEmitter) Emit(logger lager.Logger, event metric.Event) {
	if !emitter.connected {
		err := emitter.client.Connect()
		if err != nil {
			logger.Error("connection-failed", err)
			return
		}

		emitter.connected = true
	}

	err := emitter.client.SendEvent(&goryman.Event{
		Service:    emitter.servicePrefix + event.Name,
		Metric:     event.Value,
		State:      string(event.State),
		Attributes: event.Attributes,

		Host: event.Host,
		Time: event.Time,
		Tags: event.Tags,
	})
	if err != nil {
		logger.Error("failed-to-emit", err)

		if err := emitter.client.Close(); err != nil {
			logger.Error("failed-to-close", err)
		}

		emitter.connected = false
	}
}
