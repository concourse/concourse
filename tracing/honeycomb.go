package tracing

import (
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type Honeycomb struct {
	APIKey  string `long:"honeycomb-api-key" description:"honeycomb.io api key"`
	Dataset string `long:"honeycomb-dataset" description:"honeycomb.io dataset name"`
}

func (h Honeycomb) IsConfigured() bool {
	return h.APIKey != "" && h.Dataset != ""
}

func (h Honeycomb) Exporter() (sdktrace.SpanExporter, []sdktrace.TracerProviderOption, error) {
	return OTLP{
		Address: "api.honeycomb.io:443",
		Headers: map[string]string{
			"x-honeycomb-team":    h.APIKey,
			"x-honeycomb-dataset": h.Dataset,
		},
		UseTLS: true,
	}.Exporter()
}
