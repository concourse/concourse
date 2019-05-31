package radar_test

import (
	"context"
	"errors"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/concourse/concourse/v5/atc/radar"
	"github.com/concourse/concourse/v5/atc/radar/radarfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("IntervalRunner", func() {
	var (
		runAt  time.Time
		scanAt time.Time

		fakeClock *fakeclock.FakeClock
		interval  time.Duration
		runTimes  chan time.Time
		scanTimes chan time.Time

		intervalRunner    IntervalRunner
		fakeScanner       *radarfakes.FakeScanner
		fakeNotifications *radarfakes.FakeNotifications

		ctx    context.Context
		cancel context.CancelFunc
	)

	BeforeEach(func() {
		runAt = time.Unix(123, 456).UTC()
		scanAt = time.Unix(111, 111).UTC()
		fakeClock = fakeclock.NewFakeClock(runAt)

		fakeScanner = new(radarfakes.FakeScanner)
		fakeNotifications = new(radarfakes.FakeNotifications)

		runTimes = make(chan time.Time, 100)
		scanTimes = make(chan time.Time, 100)
		interval = 1 * time.Minute

		ctx, cancel = context.WithCancel(context.Background())

		logger := lagertest.NewTestLogger("test")
		intervalRunner = NewIntervalRunner(logger, fakeClock, 12, fakeScanner, fakeNotifications)
	})

	Describe("RunFunc", func() {
		var runErrs chan error

		BeforeEach(func() {
			fakeScanner.RunStub = func(lager.Logger, int) (time.Duration, error) {
				runTimes <- fakeClock.Now()
				return interval, nil
			}
			fakeScanner.ScanStub = func(lager.Logger, int) error {
				scanTimes <- scanAt
				return nil
			}
		})

		JustBeforeEach(func() {
			errs := make(chan error, 1)
			runErrs = errs
			go func() {
				errs <- intervalRunner.Run(ctx)
				close(errs)
			}()
		})

		Context("when listening for notifications fails", func() {
			BeforeEach(func() {
				fakeNotifications.ListenReturns(nil, errors.New("nope"))
			})

			It("errors", func() {
				Expect(<-runErrs).To(HaveOccurred())
			})
		})

		Context("when listening for notifications succeeds", func() {
			var notify chan bool

			BeforeEach(func() {
				notify = make(chan bool)
				fakeNotifications.ListenReturns(notify, nil)
			})

			AfterEach(func() {
				cancel()
				Expect(<-runErrs).To(BeNil())
				Expect(fakeNotifications.UnlistenCallCount()).To(Equal(1))
			})

			Context("when scanner.Run() returns an error", func() {
				var disaster = errors.New("failed")
				BeforeEach(func() {
					fakeScanner.RunStub = func(lager.Logger, int) (time.Duration, error) {
						runTimes <- fakeClock.Now()
						return interval, disaster
					}
				})

				It("returns an error", func() {
					Expect(<-runErrs).To(Equal(disaster))
				})
			})

			Context("when scanner.Run() returns ErrFailedToAcquireLock error", func() {
				BeforeEach(func() {
					fakeScanner.RunStub = func(lager.Logger, int) (time.Duration, error) {
						runTimes <- fakeClock.Now()
						return interval, ErrFailedToAcquireLock
					}
				})

				It("waits for the interval and tries again", func() {
					<-runTimes

					fakeClock.WaitForWatcherAndIncrement(interval)
					Expect(<-runTimes).To(Equal(runAt.Add(interval)))
				})
			})

			Context("when run does not return error", func() {
				It("immediately runs a scan", func() {
					Expect(<-runTimes).To(Equal(runAt))
				})

				It("runs a scan on returned interval", func() {
					Expect(<-runTimes).To(Equal(runAt))

					fakeClock.WaitForWatcherAndIncrement(interval)
					Expect(<-runTimes).To(Equal(runAt.Add(interval)))
				})

				Context("when it receives a notification", func() {
					BeforeEach(func() {
						fakeScanner.ScanStub = func(lager.Logger, int) error {
							scanTimes <- scanAt
							return nil
						}
					})

					It("triggers a Scan", func() {
						Expect(<-runTimes).To(Equal(runAt))

						notify <- true
						Expect(<-scanTimes).To(Equal(scanAt))
					})
				})

				Context("when Run takes a while", func() {
					BeforeEach(func() {
						fakeScanner.RunStub = func(lager.Logger, int) (time.Duration, error) {
							runTimes <- fakeClock.Now()
							fakeClock.Increment(interval / 2)
							return interval, nil
						}
					})

					It("starts counting interval after the process is finished", func() {
						Expect(<-runTimes).To(Equal(runAt))

						fakeClock.WaitForWatcherAndIncrement(interval / 2)
						fakeClock.Increment(interval / 2)
						Expect(<-runTimes).To(Equal(runAt.Add(interval + (interval / 2))))
					})
				})

			})
		})
	})
})
