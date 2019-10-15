package tracing

import (
	"fmt"

	"go.opentelemetry.io/exporter/trace/stdout"
	"go.opentelemetry.io/sdk/export"
)

// Stdout is an exporter that just prints the spans to the standard output.
//
type Stdout struct {
	Configured bool `long:"tracing-stdout" description:"enables printing traces to stdout"`
}

func (s Stdout) IsConfigured() bool {
	return s.Configured
}

func (s Stdout) Exporter() (export.SpanSyncer, error) {
	exporter, err := stdout.NewExporter(stdout.Options{PrettyPrint: false})
	if err != nil {
		err = fmt.Errorf("failed to create stdout exporter: %w", err)
		return nil, err
	}

	return exporter, nil
}
