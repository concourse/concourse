package tracing

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 go.opentelemetry.io/api/trace.Tracer
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 go.opentelemetry.io/api/trace.Span

import (
	"context"

	"go.opentelemetry.io/api/trace"
	"go.opentelemetry.io/sdk/export"
	sdktrace "go.opentelemetry.io/sdk/trace"
)

// StartSpan creates a span, giving back a context that has itself added as the
// parent span.
//
// Calls to this function with a context that has been generated from a previous
// call to this method will make the resulting span a child of the span that
// preceeded it.
//
// For instance:
//
// ```
// func fn () {
//
//     rootCtx, rootSpan := StartSpan(context.Background(), "foo", nil)
//     defer rootSpan.End()
//
//     _, childSpan := StartSpan(rootCtx, "bar", nil)
//     defer childSpan.End()
//
// }
// ```
//
// calling `fn()` will lead to the following trace:
//
// ```
// foo   0--------3
//   bar    1----2
// ```
//
// where (0) is the start of the root span, which then gets a child `bar`
// initializing at (1), having its end called (2), and then the last span
// finalization happening for the root span (3) given how `defer` statements
// stack.
//
func StartSpan(
	ctx context.Context,
	component string,
	attrs Attrs,
) (context.Context, trace.Span) {
	ctx, span := trace.GlobalTracer().Start(
		ctx,
		component,
	)

	span.SetAttributes(keyValueSlice(attrs)...)

	return ctx, span
}

// ConfigureTracer configures the sdk to use a given exporter.
//
// By default, a noop tracer is registered, thus, it's safe to call StartSpan
// and other related methods even before `ConfigureTracer` it called.
//
func ConfigureTracer(exporter export.SpanSyncer) {
	sdktrace.Register()

	sdktrace.RegisterSpanProcessor(
		sdktrace.NewSimpleSpanProcessor(exporter),
	)

	sdktrace.ApplyConfig(
		sdktrace.Config{
			DefaultSampler: sdktrace.AlwaysSample(),
		},
	)
}
