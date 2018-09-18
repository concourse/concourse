package radar_test

import (
	"context"
	"errors"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/concourse/atc/radar"
	"github.com/concourse/atc/radar/radarfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("IntervalRunner", func() {
	var (
		epoch time.Time

		fakeClock *fakeclock.FakeClock
		interval  time.Duration
		times     chan time.Time

		intervalRunner IntervalRunner
		fakeScanner    *radarfakes.FakeScanner

		ctx    context.Context
		cancel context.CancelFunc
	)

	BeforeEach(func() {
		epoch = time.Unix(123, 456).UTC()
		fakeClock = fakeclock.NewFakeClock(epoch)

		fakeScanner = &radarfakes.FakeScanner{}
		times = make(chan time.Time, 100)
		interval = 1 * time.Minute
		fakeScanner.RunStub = func(lager.Logger, string) (time.Duration, error) {
			times <- fakeClock.Now()
			return interval, nil
		}
		ctx, cancel = context.WithCancel(context.Background())

		logger := lagertest.NewTestLogger("test")
		intervalRunner = NewIntervalRunner(logger, fakeClock, "some-resource", fakeScanner)
	})

	Describe("RunFunc", func() {
		var runErrs chan error

		JustBeforeEach(func() {
			errs := make(chan error, 1)
			runErrs = errs
			go func() {
				errs <- intervalRunner.Run(ctx)
				close(errs)
			}()
		})

		AfterEach(func() {
			cancel()
			Expect(<-runErrs).To(BeNil())
		})

		Context("when run does not return error", func() {
			It("immediately runs a scan", func() {
				Expect(<-times).To(Equal(epoch))
			})

			It("runs a scan on returned interval", func() {
				Expect(<-times).To(Equal(epoch))

				fakeClock.WaitForWatcherAndIncrement(interval)
				Expect(<-times).To(Equal(epoch.Add(interval)))
			})

			Context("when Run takes a while", func() {
				BeforeEach(func() {
					fakeScanner.RunStub = func(lager.Logger, string) (time.Duration, error) {
						times <- fakeClock.Now()
						fakeClock.Increment(interval / 2)
						return interval, nil
					}
				})

				It("starts counting interval after the process is finished", func() {
					Expect(<-times).To(Equal(epoch))

					fakeClock.WaitForWatcherAndIncrement(interval / 2)
					fakeClock.Increment(interval / 2)
					Expect(<-times).To(Equal(epoch.Add(interval + (interval / 2))))
				})
			})
		})

		Context("when scanner.Run() returns an error", func() {
			var disaster = errors.New("failed")
			BeforeEach(func() {
				fakeScanner.RunStub = func(lager.Logger, string) (time.Duration, error) {
					times <- fakeClock.Now()
					return interval, disaster
				}
			})

			It("returns an error", func() {
				Expect(<-runErrs).To(Equal(disaster))
			})
		})

		Context("when scanner.Run() returns ErrFailedToAcquireLock error", func() {
			BeforeEach(func() {
				fakeScanner.RunStub = func(lager.Logger, string) (time.Duration, error) {
					times <- fakeClock.Now()
					return interval, ErrFailedToAcquireLock
				}
			})

			It("waits for the interval and tries again", func() {
				<-times

				fakeClock.WaitForWatcherAndIncrement(interval)
				Expect(<-times).To(Equal(epoch.Add(interval)))
			})
		})

	})
})
