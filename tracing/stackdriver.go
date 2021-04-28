package tracing

import (
	"fmt"

	texporter "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type Stackdriver struct {
	ProjectID string `long:"stackdriver-projectid" description:"GCP's Project ID"`
}

func (s Stackdriver) IsConfigured() bool {
	return s.ProjectID != ""
}

func (s Stackdriver) Exporter() (sdktrace.SpanExporter, []sdktrace.TracerProviderOption, error) {
	exporter, err := texporter.NewExporter(texporter.WithProjectID(s.ProjectID))
	if err != nil {
		err = fmt.Errorf("failed to create stackdriver exporter: %w", err)
		return nil, nil, err
	}

	return exporter, nil, nil
}
