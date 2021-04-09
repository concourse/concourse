package tracing

import (
	"errors"
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/honeycombio/opentelemetry-exporter-go/honeycomb"
	export "go.opentelemetry.io/otel/sdk/export/trace"
)

type Honeycomb struct {
	Enabled     bool   `yaml:"enabled,omitempty"`
	APIKey      string `yaml:"api_key,omitempty"`
	Dataset     string `yaml:"dataset,omitempty"`
	ServiceName string `yaml:"service_name,omitempty"`
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

func (h Honeycomb) Exporter() (export.SpanSyncer, error) {
	exporter, err := honeycomb.NewExporter(
		honeycomb.Config{
			APIKey: h.APIKey,
		},
		honeycomb.TargetingDataset(h.Dataset),
		honeycomb.WithServiceName(h.ServiceName),
	)
	if err != nil {
		err = fmt.Errorf("failed to create honeycomb exporter: %w", err)
		return nil, err
	}

	return exporter, nil
}
