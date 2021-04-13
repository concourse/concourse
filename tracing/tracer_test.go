package tracing_test

import (
	"context"
	"reflect"

	"github.com/concourse/concourse/tracing"
	"github.com/concourse/concourse/tracing/tracingfakes"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/label"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Tracer", func() {

	var (
		fakeSpan *tracingfakes.FakeSpan
	)

	BeforeEach(func() {
		fakeTracer := new(tracingfakes.FakeTracer)
		fakeProvider := new(tracingfakes.FakeProvider)
		fakeSpan = new(tracingfakes.FakeSpan)

		fakeProvider.TracerReturns(fakeTracer)

		fakeTracer.StartReturns(
			context.Background(),
			fakeSpan,
		)

		global.SetTraceProvider(fakeProvider)
		tracing.Configured = true
	})

	Describe("StartSpan", func() {

		var (
			ctx  context.Context
			span trace.Span

			component = "a"
			attrs     = tracing.Attrs{}
		)

		JustBeforeEach(func() {
			_, span = tracing.StartSpan(ctx, component, attrs)
		})

		It("creates a span", func() {
			Expect(span).ToNot(BeNil())
		})

		Context("with attributes", func() {

			BeforeEach(func() {
				attrs = tracing.Attrs{
					"foo": "bar",
					"zaz": "caz",
				}
			})

			It("sets the attributes passed in", func() {
				Expect(fakeSpan.SetAttributesCallCount()).To(Equal(1))

				attrs := fakeSpan.SetAttributesArgsForCall(0)
				Expect(attrs).To(ConsistOf([]label.KeyValue{
					label.String("foo", "bar"),
					label.String("zaz", "caz"),
				}))
			})
		})

	})

	Describe("Prepare", func() {
		BeforeEach(func() {
			tracing.Configured = false
		})

		It("configures tracing if jaeger flags are provided", func() {
			c := tracing.Config{
				Provider: "jaeger",
				Providers: tracing.ProvidersConfig{
					Jaeger: tracing.Jaeger{
						Endpoint: "http://jaeger:14268/api/traces",
					},
				},
			}
			c.Prepare()
			Expect(tracing.Configured).To(BeTrue())
		})

		It("configures tracing if otlp flags are provided", func() {
			c := tracing.Config{
				Provider: "otlp",
				Providers: tracing.ProvidersConfig{
					OTLP: tracing.OTLP{
						Address: "ingest.example.com:443",
						Headers: map[string]string{"access-token": "mytoken"},
					},
				},
			}
			c.Prepare()
			Expect(tracing.Configured).To(BeTrue())
		})

		It("does not configure tracing if no flags are provided", func() {
			c := tracing.Config{}
			c.Prepare()
			Expect(tracing.Configured).To(BeFalse())
		})
	})

	Describe("All", func() {
		var (
			expectedTracingProviders []string
			actualTracingProviders   []string
		)

		BeforeEach(func() {
			tracingProviders := tracing.ProvidersConfig{}
			v := reflect.ValueOf(tracingProviders)
			for i := 0; i < v.NumField(); i++ {
				provider := v.Field(i).Interface().(tracing.Service)
				expectedTracingProviders = append(expectedTracingProviders, provider.ID())
			}

			for name := range tracingProviders.All() {
				actualTracingProviders = append(actualTracingProviders, name)
			}
		})

		It("represents all the tracing providers that are included in ProvidersConfig", func() {
			Expect(actualTracingProviders).Should(ConsistOf(expectedTracingProviders))
		})
	})
})
