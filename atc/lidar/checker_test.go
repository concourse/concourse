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
	"github.com/concourse/concourse/atc/lidar/lidarfakes"
	"github.com/concourse/concourse/tracing"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/api/trace/testtrace"
)

type Checker interface {
	Run(context.Context) error
}

var _ = Describe("Checker", func() {
	var (
		err error

		fakeCheckFactory   *dbfakes.FakeCheckFactory
		fakeEngine         *enginefakes.FakeEngine
		fakeRateCalculator *lidarfakes.FakeRateCalculator
		fakeLimiter        *lidarfakes.FakeLimiter

		checker Checker
		logger  *lagertest.TestLogger
	)

	BeforeEach(func() {
		fakeCheckFactory = new(dbfakes.FakeCheckFactory)
		fakeEngine = new(enginefakes.FakeEngine)
		fakeRateCalculator = new(lidarfakes.FakeRateCalculator)
		fakeLimiter = new(lidarfakes.FakeLimiter)

		logger = lagertest.NewTestLogger("test")
	})

	JustBeforeEach(func() {
		checker = lidar.NewChecker(
			logger,
			fakeCheckFactory,
			fakeEngine,
			fakeRateCalculator,
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
				tracing.ConfigureTraceProvider(&tracing.TestTraceProvider{})
				fakeCheck := new(dbfakes.FakeCheck)
				fakeCheck.IDReturns(1)
				var ctx context.Context
				ctx, scanSpan = tracing.StartSpan(context.Background(), "fake-operation", nil)
				fakeCheck.SpanContextReturns(db.NewSpanContext(ctx))

				fakeCheckFactory.StartedChecksReturns([]db.Check{
					fakeCheck,
				}, nil)

				fakeLimiter.WaitReturns(nil)
				fakeRateCalculator.RateLimiterReturns(fakeLimiter, nil)

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
			var fakeCheck1, fakeCheck2, fakeCheck3 *dbfakes.FakeCheck

			BeforeEach(func() {
				fakeCheck1 = new(dbfakes.FakeCheck)
				fakeCheck1.IDReturns(1)
				fakeCheck2 = new(dbfakes.FakeCheck)
				fakeCheck2.IDReturns(2)
				fakeCheck3 = new(dbfakes.FakeCheck)
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

			Context("when the rate limiter is fetched correctly", func() {
				BeforeEach(func() {
					fakeLimiter.WaitReturns(nil)
					fakeRateCalculator.RateLimiterReturns(fakeLimiter, nil)
				})

				It("succeeds", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("runs all pending checks", func() {
					Eventually(fakeEngine.NewCheckCallCount).Should(Equal(3))
				})

				It("rate limits all the checks", func() {
					Eventually(fakeLimiter.WaitCallCount()).Should(Equal(3))
				})

				Context("when there is a manually triggered check and the rate limiter is not allowing any checks to run", func() {
					BeforeEach(func() {
						fakeCheck1.ManuallyTriggeredReturns(true)

						fakeLimiter.WaitReturns(errors.New("not-allowed"))
					})

					It("runs the manually triggered check", func() {
						Eventually(fakeEngine.NewCheckCallCount).Should(Equal(1))
						Expect(fakeEngine.NewCheckArgsForCall(0).ID()).To(Equal(fakeCheck1.ID()))
					})
				})
			})

			Context("when calculating the rate limit fails", func() {
				BeforeEach(func() {
					fakeRateCalculator.RateLimiterReturns(nil, errors.New("disaster"))
				})

				It("errors", func() {
					Expect(err).To(HaveOccurred())
				})
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

				fakeLimiter.WaitReturns(nil)
				fakeRateCalculator.RateLimiterReturns(fakeLimiter, nil)
			})

			It("succeeds", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("runs only one pending check", func() {
				Eventually(fakeEngine.NewCheckCallCount).Should(Equal(1))
			})
		})

		Context("when check run panicing", func() {
			var fakeRunnable *enginefakes.FakeRunnable
			var fakeCheck *dbfakes.FakeCheck

			BeforeEach(func() {
				fakeCheck = new(dbfakes.FakeCheck)
				fakeRunnable = new(enginefakes.FakeRunnable)

				fakeEngine.NewCheckReturns(fakeRunnable)

				fakeCheckFactory.StartedChecksReturns([]db.Check{
					fakeCheck,
				}, nil)

				fakeLimiter.WaitReturns(nil)
				fakeRateCalculator.RateLimiterReturns(fakeLimiter, nil)

				fakeRunnable.RunStub = func(context.Context) {
					panic("something went wrong")
				}
			})

			It("tries to run the runnable", func() {
				Expect(err).NotTo(HaveOccurred())
				Eventually(fakeRunnable.RunCallCount).Should(Equal(1))
				Eventually(fakeCheck.FinishWithErrorCallCount).Should(Equal(1))
				Eventually(fakeCheck.FinishWithErrorArgsForCall(0).Error).Should(ContainSubstring("something went wrong"))
			})
		})
	})
})
