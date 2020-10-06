package tracing

import (
	"go.opentelemetry.io/otel/exporters/otlp"
	export "go.opentelemetry.io/otel/sdk/export/trace"
	"google.golang.org/grpc/credentials"
)

// OTLP service to export traces to
type OTLP struct {
	Address string            `long:"otlp-address" description:"otlp address to send traces to"`
	Headers map[string]string `long:"otlp-header" description:"headers to attach to each tracing message"`
}

// IsConfigured identifies if an Address has been set
func (s OTLP) IsConfigured() bool {
	return s.Address != ""
}

// Exporter returns a SpanExporter to sync spans to OTLP
func (s OTLP) Exporter() (export.SpanSyncer, error) {
	secureOption := otlp.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, ""))

	exporter, err := otlp.NewExporter(
		secureOption,
		otlp.WithAddress(s.Address),
		otlp.WithHeaders(s.Headers),
	)

	if err != nil {
		return nil, err
	}

	return exporter, nil
}
