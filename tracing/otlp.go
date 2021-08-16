package tracing

import (
	"context"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
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

func (s OTLP) security() otlptracegrpc.Option {
	if s.UseTLS {
		return otlptracegrpc.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, ""))
	}

	return otlptracegrpc.WithInsecure()
}

// Exporter returns a SpanExporter to sync spans to OTLP
func (s OTLP) Exporter() (sdktrace.SpanExporter, []sdktrace.TracerProviderOption, error) {
	driver := otlptracegrpc.NewClient(
		otlptracegrpc.WithEndpoint(s.Address),
		otlptracegrpc.WithHeaders(s.Headers),
		s.security(),
	)
	exporter, err := otlptrace.New(context.TODO(), driver)
	if err != nil {
		return nil, nil, err
	}

	return exporter, nil, nil
}
