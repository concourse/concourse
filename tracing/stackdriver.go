package tracing

import (
	"fmt"

	"go.opentelemetry.io/otel/exporter/trace/stackdriver"
	export "go.opentelemetry.io/otel/sdk/export/trace"
)

type Stackdriver struct {
	ProjectID string `long:"stackdriver-projectid" description:"GCP's Project ID"`
}

func (s Stackdriver) IsConfigured() bool {
	return s.ProjectID != ""
}

func (s Stackdriver) Exporter() (export.SpanSyncer, error) {
	exporter, err := stackdriver.NewExporter(
		stackdriver.WithProjectID(s.ProjectID),
	)
	if err != nil {
		err = fmt.Errorf("failed to create stackdriver exporter: %w", err)
		return nil, err
	}

	return exporter, nil
}
