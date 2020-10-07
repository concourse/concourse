package tracing

import (
	"fmt"

	"go.opentelemetry.io/otel/exporters/trace/jaeger"
	export "go.opentelemetry.io/otel/sdk/export/trace"
)

// Jaeger service to export traces to
type Jaeger struct {
	Endpoint string            `long:"jaeger-endpoint" description:"jaeger http-based thrift collector"`
	Tags     map[string]string `long:"jaeger-tags"     description:"tags to add to the components"`
	Service  string            `long:"jaeger-service"  description:"jaeger process service name" default:"web"`
}

// IsConfigured identifies if an endpoint has been set
func (j Jaeger) IsConfigured() bool {
	return j.Endpoint != ""
}

// Exporter returns a SpanExporter to sync spans to Jaeger
func (j Jaeger) Exporter() (export.SpanSyncer, error) {
	exporter, err := jaeger.NewRawExporter(
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
