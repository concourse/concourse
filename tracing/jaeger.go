package tracing

import (
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/trace/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv"
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
func (j Jaeger) Exporter() (sdktrace.SpanExporter, []sdktrace.TracerProviderOption, error) {
	attributes := append([]attribute.KeyValue{semconv.ServiceNameKey.String(j.Service)}, keyValueSlice(j.Tags)...)
	exporter, err := jaeger.NewRawExporter(
		jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(j.Endpoint)),
	)
	extraOptions := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(resource.NewWithAttributes(attributes...)),
	}
	if err != nil {
		err = fmt.Errorf("failed to create jaeger exporter: %w", err)
		return nil, nil, err
	}

	return exporter, extraOptions, nil
}
