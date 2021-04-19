package tracing

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel/exporters/otlp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpgrpc"
	export "go.opentelemetry.io/otel/sdk/export/trace"
	"google.golang.org/grpc/credentials"
)

// OTLP service to export traces to
type OTLP struct {
	Address string            `yaml:"address,omitempty"`
	Headers map[string]string `yaml:"header,omitempty"`
	UseTLS  bool              `yaml:"use_tls,omitempty"`
}

func (s OTLP) ID() string {
	return "otlp"
}

// Validate identifies if an Address has been set
func (s OTLP) Validate() error {
	if s.Address == "" {
		return errors.New("address is missing")
	}

	return nil
}

func (s OTLP) security() otlpgrpc.Option {
	if s.UseTLS {
		return otlpgrpc.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, ""))
	}

	return otlpgrpc.WithInsecure()
}

// Exporter returns a SpanExporter to sync spans to OTLP
func (s OTLP) Exporter() (export.SpanExporter, error) {
	driver := otlpgrpc.NewDriver(
		otlpgrpc.WithEndpoint(s.Address),
		otlpgrpc.WithHeaders(s.Headers),
		s.security(),
	)
	exporter, err := otlp.NewExporter(context.TODO(), driver)
	if err != nil {
		return nil, err
	}

	return exporter, nil
}
