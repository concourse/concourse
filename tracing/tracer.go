package tracing

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/translator/conventions"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/propagation"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/label"
	export "go.opentelemetry.io/otel/sdk/export/trace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
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
type Service interface {
	ID() string
	Validate() error
	Exporter() (export.SpanSyncer, error)
}

var Configured bool

type Config struct {
	ServiceName string            `yaml:"service_name,omitempty"`
	Attributes  map[string]string `yaml:"attribute,omitempty"`

	Provider  string          `yaml:"provider,omitempty" validate:"tracing_provider"`
	Providers ProvidersConfig `yaml:",inline"`
}

type ProvidersConfig struct {
	Honeycomb   Honeycomb   `yaml:"honeycomb,omitempty"`
	Jaeger      Jaeger      `yaml:"jaeger,omitempty"`
	Stackdriver Stackdriver `yaml:"stackdriver,omitempty"`
	OTLP        OTLP        `yaml:"otlp,omitempty"`
}

func (p ProvidersConfig) All() map[string]Service {
	return map[string]Service{
		p.Honeycomb.ID():   &p.Honeycomb,
		p.Jaeger.ID():      &p.Jaeger,
		p.OTLP.ID():        &p.OTLP,
		p.Stackdriver.ID(): &p.Stackdriver,
	}
}

type ValidationTracingProviderError struct{}

func (e ValidationTracingProviderError) Error() string {
	var providers []string
	providersConfig := ProvidersConfig{}
	for name := range providersConfig.All() {
		providers = append(providers, name)
	}
	return fmt.Sprintf("Not a valid tracing provider. Valid options include %v.", providers)
}

func (c Config) Prepare() error {
	var configuredService Service
	for name, service := range c.Providers.All() {
		if c.Provider == name {
			configuredService = service
		}
	}

	if configuredService != nil {
		err := configuredService.Validate()
		if err != nil {
			return err
		}

		var provider trace.Provider
		provider, err = c.TraceProvider(configuredService.Exporter)
		if provider != nil {
			ConfigureTraceProvider(provider)
		}
	}

	return nil
}

func (c Config) resource() *resource.Resource {
	attributes := []label.KeyValue{
		label.String(conventions.AttributeTelemetrySDKName, "opentelemetry"),
		label.String(conventions.AttributeTelemetrySDKLanguage, "go"),
		label.String(conventions.AttributeServiceName, c.ServiceName),
	}

	for key, value := range c.Attributes {
		attributes = append(attributes, label.String(key, value))
	}

	return resource.New(attributes...)
}

func (c Config) TraceProvider(exporter func() (export.SpanSyncer, error)) (trace.Provider, error) {
	exp, err := exporter()
	if err != nil {
		return nil, err
	}

	provider, err := sdktrace.NewProvider(sdktrace.WithConfig(
		sdktrace.Config{
			DefaultSampler: sdktrace.AlwaysSample(),
		}),
		sdktrace.WithSyncer(exp),
		sdktrace.WithResource(c.resource()),
	)
	if err != nil {
		return nil, err
	}

	return provider, nil
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

func Inject(ctx context.Context, supplier propagation.HTTPSupplier) {
	trace.TraceContext{}.Inject(ctx, supplier)
}

type WithSpanContext interface {
	SpanContext() propagation.HTTPSupplier
}

func StartSpanFollowing(
	ctx context.Context,
	following WithSpanContext,
	component string,
	attrs Attrs,
) (context.Context, trace.Span) {
	if supplier := following.SpanContext(); supplier != nil {
		ctx = trace.TraceContext{}.Extract(ctx, supplier)
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
		ctx = trace.TraceContext{}.Extract(ctx, supplier)
	}
	linkedSpanContext := trace.SpanFromContext(linked).SpanContext()

	return startSpan(
		ctx,
		component,
		attrs,
		trace.LinkedTo(linkedSpanContext),
	)
}

func startSpan(
	ctx context.Context,
	component string,
	attrs Attrs,
	opts ...trace.StartOption,
) (context.Context, trace.Span) {
	if !Configured {
		return ctx, trace.NoopSpan{}
	}

	ctx, span := global.TraceProvider().Tracer("concourse").Start(
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
		span.SetStatus(codes.Internal, "")
		span.SetAttributes(
			label.String("error-message", err.Error()),
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
