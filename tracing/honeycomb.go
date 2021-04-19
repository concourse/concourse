package tracing

import (
	export "go.opentelemetry.io/otel/sdk/export/trace"
)

type Honeycomb struct {
	APIKey  string `yaml:"api_key,omitempty"`
	Dataset string `yaml:"dataset,omitempty"`
}

func (h Honeycomb) ID() string {
	return "honeycomb"
}

func (h Honeycomb) Validate() error {
	var errs *multierror.Error

	if h.APIKey == "" {
		errs = multierror.Append(errs, errors.New("api key is missing"))
	}

	if h.Dataset == "" {
		errs = multierror.Append(errs, errors.New("dataset is missing"))
	}

	return errs.ErrorOrNil()
}

func (h Honeycomb) Exporter() (export.SpanExporter, error) {
	return OTLP{
		Address: "api.honeycomb.io:443",
		Headers: map[string]string{
			"x-honeycomb-team":    h.APIKey,
			"x-honeycomb-dataset": h.Dataset,
		},
		UseTLS: true,
	}.Exporter()
}
