package tracing

import (
	"fmt"

	"github.com/honeycombio/opentelemetry-exporter-go/honeycomb"
	export "go.opentelemetry.io/otel/sdk/export/trace"
)

type Honeycomb struct {
	APIKey      string `long:"honeycomb-api-key" description:"honeycomb.io api key"`
	Dataset     string `long:"honeycomb-dataset" description:"honeycomb.io dataset name"`
	ServiceName string `long:"honeycomb-service-name" description:"honeycomb.io service name" default:"concourse"`
}

func (h Honeycomb) IsConfigured() bool {
	return h.APIKey != "" && h.Dataset != ""
}

func (h Honeycomb) Exporter() (export.SpanSyncer, error) {
	exporter, err := honeycomb.NewExporter(
		honeycomb.Config{
			APIKey: h.APIKey,
		},
		honeycomb.TargetingDataset(h.Dataset),
		honeycomb.WithServiceName(h.ServiceName),
	)
	if err != nil {
		err = fmt.Errorf("failed to create honeycomb exporter: %w", err)
		return nil, err
	}

	return exporter, nil
}
