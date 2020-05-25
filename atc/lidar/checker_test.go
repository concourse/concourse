package lidar_test

import (
	"context"
	"errors"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/engine"
	"github.com/concourse/concourse/atc/engine/enginefakes"
	"github.com/concourse/concourse/atc/lidar"
	"github.com/concourse/concourse/tracing"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/api/trace/testtrace"
)

type Checker interface {
	Run(context.Context) error
}

type testTraceProvider struct{}

func (ttp *testTraceProvider) Tracer(name string) trace.Tracer {
	return testtrace.NewTracer()
}

var _ = Describe("Checker", func() {
	var (
		err error

		fakeCheckFactory *dbfakes.FakeCheckFactory
		fakeEngine       *enginefakes.FakeEngine

		checker Checker
		logger  *lagertest.TestLogger
	)

	BeforeEach(func() {
		fakeCheckFactory = new(dbfakes.FakeCheckFactory)
		fakeEngine = new(enginefakes.FakeEngine)

		logger = lagertest.NewTestLogger("test")
		checker = lidar.NewChecker(
			logger,
			fakeCheckFactory,
			fakeEngine,
		)
	})

	JustBeforeEach(func() {
		err = checker.Run(context.TODO())
	})

	Describe("Run", func() {

		Context("when retrieving checks fails", func() {
			BeforeEach(func() {
				fakeCheckFactory.StartedChecksReturns(nil, errors.New("nope"))
			})

			It("errors", func() {
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when tracing is configured", func() {
			var scanSpan trace.Span

			BeforeEach(func() {
				tracing.ConfigureTraceProvider(&testTraceProvider{})
				fakeCheck := new(dbfakes.FakeCheck)
				fakeCheck.IDReturns(1)
				var ctx context.Context
				ctx, scanSpan = tracing.StartSpan(context.Background(), "fake-operation", nil)
				fakeCheck.SpanContextReturns(db.NewSpanContext(ctx))

				fakeCheckFactory.StartedChecksReturns([]db.Check{
					fakeCheck,
				}, nil)

				fakeEngine.NewCheckReturns(new(enginefakes.FakeRunnable))
			})

			AfterEach(func() {
				tracing.Configured = false
			})

			It("propagates span context to check step", func() {
				Eventually(fakeEngine.NewCheckCallCount).Should(Equal(1))
				ctx, _ := fakeEngine.NewCheckArgsForCall(0)
				span, ok := tracing.FromContext(ctx).(*testtrace.Span)
				Expect(ok).To(BeTrue(), "no testtrace.Span in context")
				Expect(span.ParentSpanID()).To(Equal(scanSpan.SpanContext().SpanID))
			})
		})

		Context("when retrieving checks succeeds", func() {

			BeforeEach(func() {
				fakeCheck1 := new(dbfakes.FakeCheck)
				fakeCheck1.IDReturns(1)
				fakeCheck2 := new(dbfakes.FakeCheck)
				fakeCheck2.IDReturns(2)
				fakeCheck3 := new(dbfakes.FakeCheck)
				fakeCheck3.IDReturns(3)

				fakeCheckFactory.StartedChecksReturns([]db.Check{
					fakeCheck1,
					fakeCheck2,
					fakeCheck3,
				}, nil)

				fakeEngine.NewCheckStub = func(ctx context.Context, build db.Check) engine.Runnable {
					time.Sleep(time.Second)
					return new(enginefakes.FakeRunnable)
				}
			})

			It("succeeds", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("runs all pending checks", func() {
				Eventually(fakeEngine.NewCheckCallCount).Should(Equal(3))
			})
		})

		Context("when a check is already running", func() {

			BeforeEach(func() {
				fakeCheck := new(dbfakes.FakeCheck)
				fakeCheck.IDReturns(1)

				fakeEngine.NewCheckStub = func(ctx context.Context, build db.Check) engine.Runnable {
					time.Sleep(time.Second)
					return new(enginefakes.FakeRunnable)
				}

				fakeCheckFactory.StartedChecksReturns([]db.Check{
					fakeCheck,
					fakeCheck,
				}, nil)
			})

			It("succeeds", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("runs only one pending check", func() {
				Eventually(fakeEngine.NewCheckCallCount).Should(Equal(1))
			})
		})
	})
})
