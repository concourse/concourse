package tracing

import (
	"context"

	"go.opentelemetry.io/otel/exporters/otlp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpgrpc"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc/credentials"
)

// OTLP service to export traces to
type OTLP struct {
	Address string            `long:"otlp-address" description:"otlp address to send traces to"`
	Headers map[string]string `long:"otlp-header" description:"headers to attach to each tracing message"`
	UseTLS  bool              `long:"otlp-use-tls" description:"whether to use tls or not"`
}

// IsConfigured identifies if an Address has been set
func (s OTLP) IsConfigured() bool {
	return s.Address != ""
}

func (s OTLP) security() otlpgrpc.Option {
	if s.UseTLS {
		return otlpgrpc.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, ""))
	}

	return otlpgrpc.WithInsecure()
}

// Exporter returns a SpanExporter to sync spans to OTLP
func (s OTLP) Exporter() (sdktrace.SpanExporter, []sdktrace.TracerProviderOption, error) {
	driver := otlpgrpc.NewDriver(
		otlpgrpc.WithEndpoint(s.Address),
		otlpgrpc.WithHeaders(s.Headers),
		s.security(),
	)
	exporter, err := otlp.NewExporter(context.TODO(), driver)
	if err != nil {
		return nil, nil, err
	}

	return exporter, nil, nil
}
