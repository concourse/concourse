package metrics

import (
	"fmt"
	"net/http"

	"github.com/concourse/flag"
	"go.opentelemetry.io/otel/exporter/metric/prometheus"
	"go.opentelemetry.io/otel/sdk/metric/controller/push"
)

type Prometheus struct {
	BindIP   flag.IP `long:"prometheus-bind-ip"   description:"IP address on which to listen for web traffic."`
	BindPort uint16  `long:"prometheus-bind-port" description:"Port on which to listen for HTTP traffic."`

	controller *push.Controller
}

func (p *Prometheus) Init() (err error) {
	var hf http.HandlerFunc

	p.controller, hf, err = prometheus.InstallNewPipeline(prometheus.Config{})
	if err != nil {
		err = fmt.Errorf("prometheus install new pipeline: %w")
		return
	}

	http.HandleFunc("/", hf)
	go func() {
		_ = http.ListenAndServe(
			fmt.Sprintf("%s:%d", p.BindIP.String(), p.BindPort),
			nil,
		)
	}()

	Meter = p.controller.Meter(meterName)

	return
}

func (p Prometheus) Close() error {
	p.controller.Stop()
	return nil
}

func (p Prometheus) IsConfigured() bool {
	return len(p.BindIP.IP) == 0 && p.BindPort != 0
}
