package tracing

import (
	"fmt"

	"go.opentelemetry.io/otel/exporter/trace/jaeger"
	export "go.opentelemetry.io/otel/sdk/export/trace"
)

type Jaeger struct {
	Endpoint string            `long:"jaeger-endpoint" description:"jaeger http-based thrift collector"`
	Tags     map[string]string `long:"jaeger-tags"     description:"tags to add to the components"`
	Service  string            `long:"jaeger-service"  description:"jaeger process service name" default:"web"`
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
