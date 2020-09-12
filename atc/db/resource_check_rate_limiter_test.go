package db_test

import (
	"context"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"golang.org/x/time/rate"
)

var _ = Describe("ResourceCheckRateLimiter", func() {
	var (
		checkInterval   time.Duration
		refreshInterval time.Duration
		fakeClock       *fakeclock.FakeClock

		checkableCount int

		ctx context.Context

		limiter *db.ResourceCheckRateLimiter
	)

	BeforeEach(func() {
		checkInterval = time.Minute
		refreshInterval = 5 * time.Minute
		fakeClock = fakeclock.NewFakeClock(time.Now())

		checkableCount = 0

		ctx = context.Background()

		limiter = db.NewResourceCheckRateLimiter(dbConn, checkInterval, refreshInterval, fakeClock)
	})

	wait := func(limiter *db.ResourceCheckRateLimiter) <-chan error {
		errs := make(chan error)
		go func() {
			errs <- limiter.Wait(ctx)
		}()
		return errs
	}

	createCheckable := func() {
		config, err := resourceConfigFactory.FindOrCreateResourceConfig(
			defaultWorkerResourceType.Type,
			atc.Source{"some": "source", "count": checkableCount},
			atc.VersionedResourceTypes{},
		)
		Expect(err).ToNot(HaveOccurred())

		_, err = config.FindOrCreateScope(nil)
		Expect(err).ToNot(HaveOccurred())

		checkableCount++
	}

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

		By("adjusting the limit but returning immediately for the first time")
		Expect(<-wait(limiter)).To(Succeed())
		Expect(limiter.Limit()).To(Equal(rate.Limit(float64(checkableCount) / checkInterval.Seconds())))

		done := wait(limiter)
		select {
		case <-done:
			Fail("should not have returned yet")
		case <-time.After(100 * time.Millisecond):
		}

		By("unblocking after the the rate limit elapses")
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
	})
})
