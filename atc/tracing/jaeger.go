package tracing

import (
	"fmt"

	"go.opentelemetry.io/otel/exporter/trace/jaeger"
	"go.opentelemetry.io/otel/sdk/export"
)

// Jaeger configures support for exporting traces to Jaeger.
//
type Jaeger struct {
	Endpoint string            `long:"tracing-jaeger-endpoint" description:"jaeger http-based thrift collector"`
	Tags     map[string]string `long:"tracing-jaeger-tags"     description:"tags to add to the components"`
	Service  string            `long:"tracing-jaeger-service"  description:"jaeger process service name"        default:"web"`
}

func (j Jaeger) IsConfigured() bool {
	return j.Endpoint != ""
}

func (j Jaeger) Exporter() (export.SpanSyncer, error) {
	exporter, err := jaeger.NewExporter(
		jaeger.WithCollectorEndpoint(j.Endpoint),
		jaeger.WithProcess(jaeger.Process{
			ServiceName: j.Service,
			Tags:        keyValueSlice(j.Tags),
		}),
	)
	if err != nil {
		err = fmt.Errorf("failed to create jaeger exporter: %w", err)
		return nil, err
	}

	return exporter, nil
}
