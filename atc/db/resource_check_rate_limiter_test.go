package db_test

import (
	"context"
	"fmt"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"golang.org/x/time/rate"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbtest"
)

var _ = Describe("ResourceCheckRateLimiter", func() {
	var (
		checkInterval      time.Duration
		checksPerSecond    int
		minChecksPerSecond float64
		refreshInterval    time.Duration
		fakeClock          *fakeclock.FakeClock

		scenario       *dbtest.Scenario
		checkableCount int

		ctx context.Context

		limiter *db.ResourceCheckRateLimiter

		pipelineConfig *atc.Config
	)

	BeforeEach(func() {
		checkInterval = time.Minute
		checksPerSecond = 0
		// schedule at least 2 checks per minute, regardless of the checkableCount
		minChecksPerSecond = 2.0 / 60
		refreshInterval = 5 * time.Minute
		fakeClock = fakeclock.NewFakeClock(time.Now())

		checkableCount = 0
		scenario = &dbtest.Scenario{}

		ctx = context.Background()

		By("Unpolluting our env")
		err := defaultPipeline.Destroy()
		Expect(err).NotTo(HaveOccurred())

		pipelineConfig = &atc.Config{}
	})

	JustBeforeEach(func() {
		limiter = db.NewResourceCheckRateLimiter(
			rate.Limit(checksPerSecond),
			rate.Limit(minChecksPerSecond),
			checkInterval,
			dbConn,
			refreshInterval,
			fakeClock,
		)
	})

	wait := func(limiter *db.ResourceCheckRateLimiter) <-chan error {
		errs := make(chan error)
		go func() {
			errs <- limiter.Wait(ctx)
		}()
		return errs
	}

	createCheckable := func() {
		checkableCount++

		pipelineConfig.Resources = append(pipelineConfig.Resources, atc.ResourceConfig{Name: fmt.Sprintf("resource-%d", checkableCount),
			Type:   dbtest.BaseResourceType,
			Source: atc.Source{"some": "source"},
		})

		scenario.Run(builder.WithPipeline(*pipelineConfig))
	}

	Context("with no static limit provided", func() {
		BeforeEach(func() {
			checksPerSecond = 0
		})

		It("rate limits while adjusting to the amount of checkables", func() {
			By("immediately returning with 0 checkables")
			Expect(<-wait(limiter)).To(Succeed())
			Expect(limiter.Limit()).To(Equal(rate.Inf))

			By("creating one checkable")
			createCheckable()

			By("continuing to return immediately, as the refresh interval has not elapsed")
			Expect(<-wait(limiter)).To(Succeed())
			Expect(limiter.Limit()).To(Equal(rate.Inf))

			By("waiting for the refresh interval")
			fakeClock.Increment(refreshInterval)

			By("adjusting the limit to the minimum value but returning immediately for the first time")
			Expect(<-wait(limiter)).To(Succeed())
			Expect(limiter.Limit()).To(Equal(rate.Limit(minChecksPerSecond)))

			done := wait(limiter)
			select {
			case <-done:
				Fail("should not have returned yet")
			case <-time.After(100 * time.Millisecond):
			}

			By("unblocking after the rate limit elapses")
			fakeClock.Increment(checkInterval / time.Duration(checkableCount))
			Expect(<-done).To(Succeed())

			By("creating more checkables")
			for i := 0; i < 10; i++ {
				createCheckable()
			}

			By("waiting for the refresh interval")
			fakeClock.Increment(refreshInterval)

			By("adjusting the limit but returning immediately for the first time")
			Expect(<-wait(limiter)).To(Succeed())
			Expect(limiter.Limit()).To(Equal(rate.Limit(float64(checkableCount) / checkInterval.Seconds())))

			done = wait(limiter)
			select {
			case <-done:
				Fail("should not have returned yet")
			case <-time.After(100 * time.Millisecond):
			}

			By("unblocking after the the new rate limit elapses")
			fakeClock.Increment(checkInterval / time.Duration(checkableCount))
			Expect(<-done).To(Succeed())

			By("inactiving all resources by reset the pipeline with no resource")
			scenario.Run(builder.WithPipeline(atc.Config{
				Resources: atc.ResourceConfigs{},
			}))

			By("waiting for the refresh interval")
			fakeClock.Increment(refreshInterval)

			By("returning immediately and retaining the infinite rate")
			Expect(<-wait(limiter)).To(Succeed())
			Expect(limiter.Limit()).To(Equal(rate.Limit(rate.Inf)))

			By("re-activing all resources")
			scenario.Run(builder.WithPipeline(*pipelineConfig))

			By("waiting for the refresh interval")
			fakeClock.Increment(refreshInterval)

			By("adjusting the limit but returning immediately for the first time")
			Expect(<-wait(limiter)).To(Succeed())
			Expect(limiter.Limit()).To(Equal(rate.Limit(float64(checkableCount) / checkInterval.Seconds())))

			By("pausing the pipeline")
			err := scenario.Pipeline.Pause("")
			Expect(err).ToNot(HaveOccurred())

			By("waiting for the refresh interval")
			fakeClock.Increment(refreshInterval)

			By("returning immediately and retaining the infinite rate")
			Expect(<-wait(limiter)).To(Succeed())
			Expect(limiter.Limit()).To(Equal(rate.Limit(rate.Inf)))
		})
	})

	Context("when a static checks per second value is provided", func() {
		BeforeEach(func() {
			checksPerSecond = 42
		})

		It("respects the value rather than determining it dynamically", func() {
			Expect(limiter.Limit()).To(Equal(rate.Limit(checksPerSecond)))
		})
	})

	Context("when a negative static checks per second value is provided", func() {
		BeforeEach(func() {
			checksPerSecond = -1
		})

		It("results in an infinite rate limit that ignores checkable count", func() {
			Expect(<-wait(limiter)).To(Succeed())
			Expect(limiter.Limit()).To(Equal(rate.Limit(rate.Inf)))

			By("creating a few (ignored) checkables")
			for i := 0; i < 10; i++ {
				createCheckable()
			}

			By("waiting for the (ignored) refresh interval")
			fakeClock.Increment(refreshInterval)

			By("still returning immediately and retaining the infinite rate")
			Expect(<-wait(limiter)).To(Succeed())
			Expect(limiter.Limit()).To(Equal(rate.Limit(rate.Inf)))
		})
	})
})
