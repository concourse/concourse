package tracing

import (
	"context"

	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/key"
	"go.opentelemetry.io/otel/api/propagators"
	"go.opentelemetry.io/otel/api/trace"
	export "go.opentelemetry.io/otel/sdk/export/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc/codes"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 go.opentelemetry.io/otel/api/trace.Tracer
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 go.opentelemetry.io/otel/api/trace.Provider
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 go.opentelemetry.io/otel/api/trace.Span

// Configured indicates whether tracing has been configured or not.
//
// This variable is needed in order to shortcircuit span generation when
// tracing hasn't been configured.
//
//
var Configured bool

type Config struct {
	Jaeger      Jaeger
	Stackdriver Stackdriver
}

func (c Config) Prepare() error {
	var exp export.SpanSyncer
	var err error
	switch {
	case c.Jaeger.IsConfigured():
		exp, err = c.Jaeger.Exporter()
	case c.Stackdriver.IsConfigured():
		exp, err = c.Stackdriver.Exporter()
	}
	if err != nil {
		return err
	}
	if exp != nil {
		ConfigureTraceProvider(TraceProvider(exp))
	}
	return nil
}

// StartSpan creates a span, giving back a context that has itself added as the
// parent span.
//
// Calls to this function with a context that has been generated from a previous
// call to this method will make the resulting span a child of the span that
// preceded it.
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
	if !Configured {
		return ctx, trace.NoopSpan{}
	}

	ctx, span := global.TraceProvider().Tracer("concourse").Start(
		ctx,
		component,
	)

	if len(attrs) != 0 {
		span.SetAttributes(keyValueSlice(attrs)...)
	}

	return ctx, span
}

func FromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

func Inject(ctx context.Context, supplier propagators.Supplier) {
	propagators.TraceContext{}.Inject(ctx, supplier)
}

type WithSpanContext interface {
	SpanContext() propagators.Supplier
}

func StartSpanFollowing(following WithSpanContext, component string, attrs Attrs) (context.Context, trace.Span) {
	if !Configured {
		return context.Background(), trace.NoopSpan{}
	}

	spanContext, _ := propagators.TraceContext{}.Extract(
		context.TODO(),
		following.SpanContext(),
	)

	ctx, span := global.TraceProvider().Tracer("concourse").Start(
		context.Background(),
		component,
		trace.FollowsFrom(spanContext),
	)

	if len(attrs) != 0 {
		span.SetAttributes(keyValueSlice(attrs)...)
	}

	return ctx, span
}

func End(span trace.Span, err error) {
	if !Configured {
		return
	}

	if err != nil {
		span.SetStatus(codes.Internal)
		span.SetAttributes(
			key.New("error").String(err.Error()),
		)
	}

	span.End()
}

// ConfigureTraceProvider configures the sdk to use a given trace provider.
//
// By default, a noop tracer is registered, thus, it's safe to call StartSpan
// and other related methods even before `ConfigureTracer` it called.
//
func ConfigureTraceProvider(tp trace.Provider) {
	global.SetTraceProvider(tp)
	Configured = true
}

func TraceProvider(exporter export.SpanSyncer) trace.Provider {
	// the only way NewProvider can error is if exporter is nil, but
	// this method is never called in such circumstances.
	provider, _ := sdktrace.NewProvider(sdktrace.WithConfig(
		sdktrace.Config{
			DefaultSampler: sdktrace.AlwaysSample(),
		}),
		sdktrace.WithSyncer(exporter),
	)
	return provider
}
