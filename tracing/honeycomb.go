package tracing

import (
	"fmt"

	"github.com/honeycombio/opentelemetry-exporter-go/honeycomb"
	export "go.opentelemetry.io/otel/sdk/export/trace"
)

type Honeycomb struct {
	APIKey      string `yaml:"api_key"`
	Dataset     string `yaml:"dataset"`
	ServiceName string `yaml:"service_name"`
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
