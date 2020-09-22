package metrics

import (
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel/exporters/metric/dogstatsd"
	"go.opentelemetry.io/otel/sdk/metric/controller/push"
)

type Dogstatsd struct {
	URL   string `long:"dogstatsd-url"   description:"dogstatsd agent url"`
	Debug bool   `long:"dogstatsd-debug" description:"writes measurements to stdout"`

	controller *push.Controller
}

func (d *Dogstatsd) Init() (err error) {
	config := dogstatsd.Config{URL: d.URL}
	if d.Debug {
		config = dogstatsd.Config{Writer: os.Stdout}
	}

	d.controller, err = dogstatsd.NewExportPipeline(config, 1*time.Minute)
	if err != nil {
		err = fmt.Errorf("dogstatsd initialization: %w", err)
		return
	}

	Meter = d.controller.Meter(meterName)
	return
}

func (d Dogstatsd) Close() error {
	d.controller.Stop()
	return nil

}

func (d Dogstatsd) IsConfigured() bool {
	return d.URL != "" || d.Debug == true
}
