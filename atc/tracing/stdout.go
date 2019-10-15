package tracing

import (
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

func (s Stdout) Exporter() export.SpanSyncer {

	// NewExporter is hardcoded to return a `nil` error, so we're safe to
	// ignore it.
	//
	exporter, _ := stdout.NewExporter(stdout.Options{PrettyPrint: false})

	return exporter
}
