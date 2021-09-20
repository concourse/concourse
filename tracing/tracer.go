package tracing

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

// Configured indicates whether tracing has been configured or not.
//
// This variable is needed in order to shortcircuit span generation when
// tracing hasn't been configured.
//
//
var Configured bool

type Config struct {
	//TODO: set default value in resective cmd package (web, worker)
	ServiceName string            `long:"service-name"  description:"service name to attach to traces as metadata" default:"concourse-web"`
	Attributes  map[string]string `long:"attribute"  description:"attributes to attach to traces as metadata"`
	Honeycomb   Honeycomb
	Jaeger      Jaeger
	Stackdriver Stackdriver
	OTLP        OTLP
}

func (c Config) Prepare() error {
	var provider trace.TracerProvider
	var err error

	switch {
	case c.Honeycomb.IsConfigured():
		provider, err = c.traceProvider(c.Honeycomb.Exporter)
	case c.Jaeger.IsConfigured():
		provider, err = c.traceProvider(c.Jaeger.Exporter)
	case c.OTLP.IsConfigured():
		provider, err = c.traceProvider(c.OTLP.Exporter)
	case c.Stackdriver.IsConfigured():
		provider, err = c.traceProvider(c.Stackdriver.Exporter)
	}
	if err != nil {
		return err
	}

	if provider != nil {
		ConfigureTraceProvider(provider)
	}

	return nil
}

func (c Config) traceProvider(exporter func() (sdktrace.SpanExporter, []sdktrace.TracerProviderOption, error)) (trace.TracerProvider, error) {
	exp, exporterOptions, err := exporter()
	if err != nil {
		return nil, err
	}

	options := append([]sdktrace.TracerProviderOption{
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSyncer(exp),
		sdktrace.WithResource(c.resource()),
	}, exporterOptions...)

	provider := sdktrace.NewTracerProvider(options...)
	if err != nil {
		return nil, err
	}

	return provider, nil
}

func (c Config) resource() *resource.Resource {
	attributes := []attribute.KeyValue{
		semconv.TelemetrySDKNameKey.String("opentelemetry"),
		semconv.TelemetrySDKLanguageKey.String("go"),
		semconv.ServiceNameKey.String(c.ServiceName),
	}

	for key, value := range c.Attributes {
		attributes = append(attributes, attribute.String(key, value))
	}

	return resource.NewSchemaless(attributes...)
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
	return startSpan(ctx, component, attrs)
}

func FromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

func Inject(ctx context.Context, carrier propagation.TextMapCarrier) {
	propagation.TraceContext{}.Inject(ctx, carrier)
}

func Extract(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	return propagation.TraceContext{}.Extract(ctx, carrier)
}

type WithSpanContext interface {
	SpanContext() propagation.TextMapCarrier
}

func StartSpanFollowing(
	ctx context.Context,
	following WithSpanContext,
	component string,
	attrs Attrs,
) (context.Context, trace.Span) {
	if supplier := following.SpanContext(); supplier != nil {
		ctx = propagation.TraceContext{}.Extract(ctx, supplier)
	}

	return startSpan(ctx, component, attrs)
}

func StartSpanLinkedToFollowing(
	linked context.Context,
	following WithSpanContext,
	component string,
	attrs Attrs,
) (context.Context, trace.Span) {
	ctx := context.Background()
	if supplier := following.SpanContext(); supplier != nil {
		ctx = propagation.TraceContext{}.Extract(ctx, supplier)
	}
	linkedSpanContext := trace.SpanFromContext(linked).SpanContext()

	return startSpan(
		ctx,
		component,
		attrs,
		trace.WithLinks(trace.Link{SpanContext: linkedSpanContext}),
	)
}

func startSpan(
	ctx context.Context,
	component string,
	attrs Attrs,
	opts ...trace.SpanStartOption,
) (context.Context, trace.Span) {
	if !Configured {
		return ctx, NoopSpan
	}

	ctx, span := otel.GetTracerProvider().Tracer("concourse").Start(
		ctx,
		component,
		opts...,
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
		span.SetStatus(codes.Error, "")
		span.SetAttributes(
			attribute.String("error-message", err.Error()),
		)
	}

	span.End()
}

// ConfigureTraceProvider configures the sdk to use a given trace provider.
//
// By default, a noop tracer is registered, thus, it's safe to call StartSpan
// and other related methods even before `ConfigureTracer` it called.
//
func ConfigureTraceProvider(tp trace.TracerProvider) {
	otel.SetTracerProvider(tp)
	Configured = true
}
