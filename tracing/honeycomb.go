package tracing

import (
	"errors"

	multierror "github.com/hashicorp/go-multierror"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
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
