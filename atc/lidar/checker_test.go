package lidar_test

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/lager"
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

		fakeCheckFactory  *dbfakes.FakeCheckFactory
		fakeEngine        *enginefakes.FakeEngine
		maxChecksInFlight uint64

		checker Checker
		logger  *lagertest.TestLogger
	)

	BeforeEach(func() {
		fakeCheckFactory = new(dbfakes.FakeCheckFactory)
		fakeEngine = new(enginefakes.FakeEngine)
		maxChecksInFlight = 10

		logger = lagertest.NewTestLogger("test")
	})

	JustBeforeEach(func() {
		checker = lidar.NewChecker(
			logger,
			fakeCheckFactory,
			fakeEngine,
			maxChecksInFlight,
		)

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
			var (
				scanSpan     trace.Span
				fakeRunnable *enginefakes.FakeRunnable
			)

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

				fakeRunnable = new(enginefakes.FakeRunnable)
				fakeEngine.NewCheckReturns(fakeRunnable)
			})

			AfterEach(func() {
				tracing.Configured = false
			})

			It("propagates span context to check step", func() {
				Eventually(fakeRunnable.RunCallCount).Should(Equal(1))
				ctx := fakeRunnable.RunArgsForCall(0)
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

				fakeEngine.NewCheckStub = func(check db.Check) engine.Runnable {
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

				fakeEngine.NewCheckStub = func(build db.Check) engine.Runnable {
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

		Context("when the maximum number of checks are reached", func() {
			BeforeEach(func() {
				var checks []db.Check

				for i := 1; i <= 10; i++ {
					fakeCheck := new(dbfakes.FakeCheck)
					fakeCheck.IDReturns(i)
					checks = append(checks, fakeCheck)
				}

				fakeCheckFactory.StartedChecksReturns(checks, nil)

				var inFlight int64
				inFlight = 0

				fakeRunnable := new(enginefakes.FakeRunnable)
				fakeEngine.NewCheckReturns(fakeRunnable)
				fakeRunnable.RunStub = func(logger lager.Logger) {
					defer GinkgoRecover()
					num := atomic.AddInt64(&inFlight, 1)
					defer atomic.AddInt64(&inFlight, -1)

					Expect(num).To(BeNumerically("<=", 5))

					time.Sleep(100 * time.Millisecond)
				}

				maxChecksInFlight = 5
			})

			It("runs all the checks within the max in flight", func() {
				Expect(err).NotTo(HaveOccurred())
				Eventually(fakeEngine.NewCheckCallCount).Should(Equal(10))
			})
		})
	})
})
