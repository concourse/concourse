package tracing

import (
	"errors"
	"fmt"

	texporter "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type Stackdriver struct {
	ProjectID string `yaml:"projectid,omitempty"`
}

func (s Stackdriver) ID() string {
	return "stackdriver"
}

func (s Stackdriver) Validate() error {
	if s.ProjectID == "" {
		return errors.New("project ID is missing")
	}

	return nil
}

func (s Stackdriver) Exporter() (sdktrace.SpanExporter, []sdktrace.TracerProviderOption, error) {
	exporter, err := texporter.NewExporter(texporter.WithProjectID(s.ProjectID))
	if err != nil {
		err = fmt.Errorf("failed to create stackdriver exporter: %w", err)
		return nil, nil, err
	}

	return exporter, nil, nil
}
