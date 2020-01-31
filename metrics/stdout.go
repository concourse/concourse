package metrics

import (
	"fmt"
	"os"

	"go.opentelemetry.io/otel/exporter/metric/stdout"
	"go.opentelemetry.io/otel/sdk/metric/controller/push"
)

type Stdout struct {
	Enabled bool `long:"stdout" description:"send metrics to stdout"`

	controller *push.Controller
}

func (s Stdout) IsConfigured() bool {
	return s.Enabled
}

func (p *Stdout) Init() (err error) {
	p.controller, err = stdout.NewExportPipeline(stdout.Config{
		Writer: os.Stdout,
	})
	if err != nil {
		err = fmt.Errorf("stdout init: %w", err)
		return
	}

	Meter = p.controller.Meter("concourse")

	return
}

func (p Stdout) Close() error {
	return nil
}
