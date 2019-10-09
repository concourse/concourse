package lidar_test

import (
	"context"
	"errors"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/db/lock/lockfakes"
	"github.com/concourse/concourse/atc/lidar"
	"github.com/concourse/concourse/atc/lidar/lidarfakes"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("IntervalRunner", func() {
	var (
		intervalRunner ifrit.Runner
		process        ifrit.Process

		logger               *lagertest.TestLogger
		fakeRunner           *lidarfakes.FakeRunner
		fakeComponent        *dbfakes.FakeComponent
		fakeLockFactory      *lockfakes.FakeLockFactory
		fakeComponentFactory *dbfakes.FakeComponentFactory
		fakeNotifications    *lidarfakes.FakeNotifications
		fakeClock            *fakeclock.FakeClock
		fakeLock             *lockfakes.FakeLock

		notifier chan bool
		runTimes chan time.Time
		interval = time.Minute
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		notifier = make(chan bool, 1)
		fakeRunner = new(lidarfakes.FakeRunner)
		fakeNotifications = new(lidarfakes.FakeNotifications)
		fakeNotifications.ListenReturns(notifier, nil)
		fakeLockFactory = new(lockfakes.FakeLockFactory)
		fakeLock = new(lockfakes.FakeLock)
		fakeComponent = new(dbfakes.FakeComponent)
		fakeComponentFactory = new(dbfakes.FakeComponentFactory)
		fakeComponentFactory.FindReturns(fakeComponent, true, nil)
		fakeClock = fakeclock.NewFakeClock(time.Unix(0, 123))

		runTimes = make(chan time.Time, 1)
		fakeRunner.RunStub = func(ctx context.Context) error {
			runTimes <- fakeClock.Now()
			return nil
		}

		intervalRunner = lidar.NewIntervalRunner(
			logger,
			fakeClock,
			fakeRunner,
			interval,
			fakeNotifications,
			"some-channel",
			fakeLockFactory,
			fakeComponentFactory,
		)
	})

	JustBeforeEach(func() {
		process = ifrit.Invoke(intervalRunner)
	})

	AfterEach(func() {
		process.Signal(os.Interrupt)
		Eventually(process.Wait()).Should(Receive())
	})

	Context("when the interval elapses", func() {

		JustBeforeEach(func() {
			fakeClock.WaitForWatcherAndIncrement(interval)
		})

		Context("when the component is paused", func() {
			BeforeEach(func() {
				fakeComponent.PausedReturns(true)
			})

			It("does not run", func() {
				Consistently(fakeRunner.RunCallCount).Should(Equal(0))
			})
		})

		Context("when the component is unpaused", func() {
			BeforeEach(func() {
				fakeComponent.PausedReturns(false)
			})

			Context("when the interval has not elapsed", func() {
				BeforeEach(func() {
					fakeComponent.IntervalElapsedReturns(false)
				})

				It("does not run", func() {
					Consistently(fakeRunner.RunCallCount).Should(Equal(0))
				})
			})

			Context("when the interval has elapsed", func() {
				BeforeEach(func() {
					fakeComponent.IntervalElapsedReturns(true)
				})

				It("calls to get a lock for component", func() {
					Eventually(fakeLockFactory.AcquireCallCount).Should(Equal(1))
					_, lockID := fakeLockFactory.AcquireArgsForCall(0)
					Expect(lockID).To(Equal(lock.NewTaskLockID("some-channel")))
				})

				Context("when getting a lock succeeds", func() {
					BeforeEach(func() {
						fakeLockFactory.AcquireReturns(fakeLock, true, nil)
					})

					It("runs", func() {
						Eventually(fakeRunner.RunCallCount).Should(Equal(1))
					})

					It("updates last ran", func() {
						Eventually(fakeComponent.UpdateLastRanCallCount).Should(Equal(1))
					})
				})

				Context("when getting a lock fails", func() {
					Context("because of an error", func() {
						BeforeEach(func() {
							fakeLockFactory.AcquireReturns(nil, true, errors.New("disaster"))
						})

						It("does not run", func() {
							Consistently(fakeRunner.RunCallCount).Should(Equal(0))
							Consistently(process.Wait()).ShouldNot(Receive())
						})

						It("does not update last ran", func() {
							Consistently(fakeComponent.UpdateLastRanCallCount).Should(Equal(0))
						})
					})

					Context("because we got acquired of false", func() {
						BeforeEach(func() {
							fakeLockFactory.AcquireReturns(nil, false, nil)
						})

						It("does not update last ran", func() {
							Consistently(fakeComponent.UpdateLastRanCallCount).Should(Equal(0))
						})
					})
				})
			})
		})
	})

	Context("when it receives a notification", func() {
		BeforeEach(func() {
			notifier <- true
		})

		Context("when the component is paused", func() {
			BeforeEach(func() {
				fakeComponent.PausedReturns(true)
			})

			It("does not run", func() {
				Consistently(fakeRunner.RunCallCount).Should(Equal(0))
			})
		})

		Context("when the component is unpaused", func() {
			BeforeEach(func() {
				fakeComponent.PausedReturns(false)
			})

			It("runs", func() {
				Eventually(fakeRunner.RunCallCount).Should(Equal(1))
			})

			It("updates last ran", func() {
				Eventually(fakeComponent.UpdateLastRanCallCount).Should(Equal(1))
			})
		})
	})
})
