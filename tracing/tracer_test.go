package tracing_test

import (
	"context"

	"github.com/concourse/concourse/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/oteltest"
	"go.opentelemetry.io/otel/trace"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Tracer", func() {
	var (
		spanRecorder *oteltest.SpanRecorder
	)

	BeforeEach(func() {
		spanRecorder = new(oteltest.SpanRecorder)

		provider := oteltest.NewTracerProvider(oteltest.WithSpanRecorder(spanRecorder))
		tracing.ConfigureTraceProvider(provider)
	})

	Describe("StartSpan", func() {
		var (
			span trace.Span

			component = "a"
			attrs     = tracing.Attrs{}
		)

		JustBeforeEach(func() {
			_, span = tracing.StartSpan(context.Background(), component, attrs)
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
				spans := spanRecorder.Started()
				Expect(spans).To(HaveLen(1))
				Expect(spans[0].Attributes()).To(Equal(map[attribute.Key]attribute.Value{
					"foo": attribute.StringValue("bar"),
					"zaz": attribute.StringValue("caz"),
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
				Jaeger: tracing.Jaeger{
					Endpoint: "http://jaeger:14268/api/traces",
				},
			}
			c.Prepare()
			Expect(tracing.Configured).To(BeTrue())
		})

		It("configures tracing if otlp flags are provided", func() {
			c := tracing.Config{
				OTLP: tracing.OTLP{
					Address: "ingest.example.com:443",
					Headers: map[string]string{"access-token": "mytoken"},
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
})
