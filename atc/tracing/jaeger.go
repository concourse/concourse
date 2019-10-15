package tracing

import (
	"fmt"

	"go.opentelemetry.io/exporter/trace/jaeger"
	"go.opentelemetry.io/sdk/export"
)

// Jaeger configures support for exporting traces to Jaeger.
//
// TODO allow configuring tags
//      https://github.com/open-telemetry/opentelemetry-go/issues/202
//
type Jaeger struct {
	Endpoint string `long:"tracing-jaeger-endpoint" description:"jaeger http-based thrift collector"`
	Service  string `long:"tracing-jaeger-service"  description:"jaeger process service name"        default:"web"`
}

func (j Jaeger) IsConfigured() bool {
	return j.Endpoint != ""
}

func (j Jaeger) Exporter() (export.SpanSyncer, error) {
	exporter, err := jaeger.NewExporter(
		jaeger.WithCollectorEndpoint(j.Endpoint),
		jaeger.WithProcess(jaeger.Process{
			ServiceName: j.Service,
		}),
	)
	if err != nil {
		err = fmt.Errorf("failed to create jaeger exporter: %w", err)
		return nil, err
	}

	return exporter, nil
}
