package radar_test

import (
	"errors"
	"os"
	"time"

	"github.com/pivotal-golang/clock/fakeclock"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/concourse/atc/radar"
	"github.com/concourse/atc/radar/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("IntervalRunner", func() {
	var (
		epoch time.Time

		fakeClock *fakeclock.FakeClock
		interval  time.Duration
		times     chan time.Time

		intervalRunner *IntervalRunner
		fakeScanner    *fakes.FakeScanner

		signalCh chan os.Signal
		doneCh   chan struct{}
		readyCh  chan struct{}
		errCh    chan error
	)

	BeforeEach(func() {
		signalCh = make(chan os.Signal)
		readyCh = make(chan struct{})
		doneCh = make(chan struct{})
		errCh = make(chan error)

		epoch = time.Unix(123, 456).UTC()
		fakeClock = fakeclock.NewFakeClock(epoch)

		fakeScanner = &fakes.FakeScanner{}
		times = make(chan time.Time, 100)
		interval = 1 * time.Minute
		fakeScanner.RunStub = func(lager.Logger, string) (time.Duration, error) {
			times <- fakeClock.Now()
			return interval, nil
		}

		logger := lagertest.NewTestLogger("test")
		intervalRunner = NewIntervalRunner(logger, fakeClock, "some-resource", fakeScanner)
	})

	Describe("RunFunc", func() {
		JustBeforeEach(func() {
			go func() {
				errCh <- intervalRunner.RunFunc(signalCh, readyCh)
				doneCh <- struct{}{}
			}()
			<-readyCh
		})

		AfterEach(func() {
			select {
			case <-doneCh:
			case signalCh <- os.Interrupt:
				<-errCh
			}
		})

		It("closes the ready channel immediately", func() {
			Expect(readyCh).To(BeClosed())
		})

		It("immediately runs a scan", func() {
			Expect(<-times).To(Equal(epoch))
		})

		It("runs a scan on returned interval", func() {
			Expect(<-times).To(Equal(epoch))

			fakeClock.WaitForWatcherAndIncrement(interval)
			Expect(<-times).To(Equal(epoch.Add(interval)))
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
				Expect(<-errCh).To(Equal(disaster))
			})
		})

		Context("when scanner.Run() returns ErrFailedToAcquireLease error", func() {
			BeforeEach(func() {
				fakeScanner.RunStub = func(lager.Logger, string) (time.Duration, error) {
					times <- fakeClock.Now()
					return interval, ErrFailedToAcquireLease
				}
			})

			It("waits for the interval and tries again", func() {
				<-times

				fakeClock.WaitForWatcherAndIncrement(interval)
				Expect(<-times).To(Equal(epoch.Add(interval)))
			})
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
})
