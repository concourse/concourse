package tracing

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

var NoopSpan trace.Span

func init() {
	tracer := trace.NewNoopTracerProvider().Tracer("")
	_, NoopSpan = tracer.Start(context.Background(), "")
}
