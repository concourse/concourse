package lidar_test

import (
	"errors"
	"time"

	"github.com/concourse/concourse/atc/lidar"
	"github.com/concourse/concourse/atc/lidar/lidarfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"golang.org/x/time/rate"
)

var _ = Describe("CheckRateCalculator", func() {
	var (
		maxChecksPerSecond       int
		resourceCheckingInterval time.Duration
		fakeCheckableCounter     *lidarfakes.FakeCheckableCounter
		checkRateCalculator      lidar.CheckRateCalculator
		rateLimit                lidar.Limiter
		calcErr                  error
	)

	BeforeEach(func() {
		fakeCheckableCounter = new(lidarfakes.FakeCheckableCounter)
		fakeCheckableCounter.CheckableCountReturns(600, nil)
		resourceCheckingInterval = 1 * time.Minute
	})

	JustBeforeEach(func() {
		checkRateCalculator = lidar.CheckRateCalculator{
			MaxChecksPerSecond:       maxChecksPerSecond,
			ResourceCheckingInterval: resourceCheckingInterval,
			CheckableCounter:         fakeCheckableCounter,
		}

		rateLimit, calcErr = checkRateCalculator.RateLimiter()
	})

	Context("when max checks per second is -1", func() {
		BeforeEach(func() {
			maxChecksPerSecond = -1
		})

		It("returns unlimited checks per second", func() {
			Expect(calcErr).ToNot(HaveOccurred())
			Expect(rateLimit).To(Equal(rate.NewLimiter(rate.Inf, 1)))
		})
	})

	Context("when max checks per second is 0", func() {
		BeforeEach(func() {
			maxChecksPerSecond = 0
		})

		It("calulates rate limit using the number of checkables and resource checking interval", func() {
			Expect(calcErr).ToNot(HaveOccurred())
			Expect(rateLimit).To(Equal(rate.NewLimiter(rate.Limit(10), 1)))
		})

		Context("when fetching the checkable count errors", func() {
			BeforeEach(func() {
				fakeCheckableCounter.CheckableCountReturns(0, errors.New("disaster"))
			})

			It("returns the error", func() {
				Expect(calcErr).To(HaveOccurred())
				Expect(calcErr).To(Equal(errors.New("disaster")))
			})
		})
	})

	Context("when max checks per second is greater than 0", func() {
		BeforeEach(func() {
			maxChecksPerSecond = 2
		})

		It("sets the rate limit to the max checks per second", func() {
			Expect(calcErr).ToNot(HaveOccurred())
			Expect(rateLimit).To(Equal(rate.NewLimiter(rate.Limit(2), 1)))
		})
	})
})
