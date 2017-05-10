package emitter

import (
	"fmt"
	"net"

	"code.cloudfoundry.org/lager"
	"github.com/The-Cloud-Source/goryman"
	"github.com/concourse/atc/metric"
)

type RiemannEmitter struct {
	client    *goryman.GorymanClient
	connected bool

	servicePrefix string
}

type RiemannConfig struct {
	Host          string `long:"riemann-host"                          description:"Riemann server address to emit metrics to."`
	Port          uint16 `long:"riemann-port"           default:"5555" description:"Port of the Riemann server to emit metrics to."`
	ServicePrefix string `long:"riemann-service-prefix" default:""     description:"An optional prefix for emitted Riemann services"`
}

func init() {
	metric.RegisterEmitter(&RiemannConfig{})
}

func (config *RiemannConfig) Description() string { return "Riemann" }
func (config *RiemannConfig) IsConfigured() bool  { return config.Host != "" }

func (config *RiemannConfig) NewEmitter() metric.Emitter {
	return &RiemannEmitter{
		client:    goryman.NewGorymanClient(net.JoinHostPort(config.Host, fmt.Sprintf("%d", config.Port))),
		connected: false,

		servicePrefix: config.ServicePrefix,
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
