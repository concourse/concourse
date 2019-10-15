package tracing_test

import (
	"context"

	"github.com/concourse/concourse/atc/tracing"
	"github.com/concourse/concourse/atc/tracing/tracingfakes"
	"go.opentelemetry.io/api/core"
	"go.opentelemetry.io/api/key"
	"go.opentelemetry.io/api/trace"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Tracer", func() {

	var (
		fakeSpan *tracingfakes.FakeSpan
	)

	BeforeEach(func() {
		fakeTracer := new(tracingfakes.FakeTracer)
		fakeSpan = new(tracingfakes.FakeSpan)

		fakeTracer.StartReturns(
			context.Background(),
			fakeSpan,
		)

		trace.SetGlobalTracer(fakeTracer)
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
				Expect(attrs).To(ConsistOf([]core.KeyValue{
					key.New("foo").String("bar"),
					key.New("zaz").String("caz"),
				}))
			})
		})

	})

})
