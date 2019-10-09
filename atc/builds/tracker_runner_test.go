package builds_test

import (
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"

	. "github.com/concourse/concourse/atc/builds"
	"github.com/concourse/concourse/atc/builds/buildsfakes"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/db/lock/lockfakes"
)

var _ = Describe("TrackerRunner", func() {
	var (
		trackerRunner TrackerRunner
		process       ifrit.Process

		logger               *lagertest.TestLogger
		fakeTracker          *buildsfakes.FakeBuildTracker
		fakeComponent        *dbfakes.FakeComponent
		fakeLockFactory      *lockfakes.FakeLockFactory
		fakeComponentFactory *dbfakes.FakeComponentFactory
		fakeNotifications    *buildsfakes.FakeNotifications
		fakeClock            *fakeclock.FakeClock
		fakeLock             *lockfakes.FakeLock

		shutdownNotify     chan bool
		buildStartedNotify chan bool
		trackTimes         chan time.Time
		interval           = time.Minute
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		fakeTracker = new(buildsfakes.FakeBuildTracker)
		fakeNotifications = new(buildsfakes.FakeNotifications)
		fakeComponent = new(dbfakes.FakeComponent)
		fakeComponentFactory = new(dbfakes.FakeComponentFactory)
		fakeComponentFactory.FindReturns(fakeComponent, true, nil)
		fakeLockFactory = new(lockfakes.FakeLockFactory)
		fakeLock = new(lockfakes.FakeLock)
		fakeClock = fakeclock.NewFakeClock(time.Unix(0, 123))

		trackTimes = make(chan time.Time, 1)
		fakeTracker.TrackStub = func() {
			trackTimes <- fakeClock.Now()
		}

		shutdownNotify = make(chan bool, 1)
		buildStartedNotify = make(chan bool, 1)

		fakeNotifications.ListenReturnsOnCall(0, shutdownNotify, nil)
		fakeNotifications.ListenReturnsOnCall(1, buildStartedNotify, nil)

		trackerRunner = TrackerRunner{
			Tracker:          fakeTracker,
			Notifications:    fakeNotifications,
			Interval:         interval,
			Clock:            fakeClock,
			Logger:           logger,
			LockFactory:      fakeLockFactory,
			ComponentFactory: fakeComponentFactory,
		}
	})

	JustBeforeEach(func() {
		process = ifrit.Invoke(trackerRunner)
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
				Consistently(fakeTracker.TrackCallCount).Should(Equal(0))
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
					Consistently(fakeTracker.TrackCallCount).Should(Equal(0))
				})
			})

			Context("when the interval has elapsed", func() {
				BeforeEach(func() {
					fakeComponent.IntervalElapsedReturns(true)
				})

				It("calls to get a lock for component", func() {
					Eventually(fakeLockFactory.AcquireCallCount).Should(Equal(1))
					_, lockID := fakeLockFactory.AcquireArgsForCall(0)
					Expect(lockID).To(Equal(lock.NewTaskLockID("build-tracker")))
				})

				Context("when getting a lock succeeds", func() {
					BeforeEach(func() {
						fakeLockFactory.AcquireReturns(fakeLock, true, nil)
					})
					It("tracks", func() {
						Eventually(fakeTracker.TrackCallCount).Should(Equal(1))
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
							Eventually(fakeTracker.TrackCallCount).Should(Equal(0))
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

	Context("when it receives an ATC shutdown notice", func() {
		BeforeEach(func() {
			shutdownNotify <- true
		})

		Context("when the component is paused", func() {
			BeforeEach(func() {
				fakeComponent.PausedReturns(true)
			})

			It("does not run", func() {
				Consistently(fakeTracker.TrackCallCount).Should(Equal(0))
			})
		})

		Context("when the component is unpaused", func() {
			BeforeEach(func() {
				fakeComponent.PausedReturns(false)
			})

			It("tracks", func() {
				Eventually(fakeTracker.TrackCallCount).Should(Equal(1))
			})
		})
	})

	Context("when it receives a build started notice", func() {
		BeforeEach(func() {
			buildStartedNotify <- true
		})

		Context("when the component is paused", func() {
			BeforeEach(func() {
				fakeComponent.PausedReturns(true)
			})

			It("does not run", func() {
				Consistently(fakeTracker.TrackCallCount).Should(Equal(0))
			})
		})

		Context("when the component is unpaused", func() {
			BeforeEach(func() {
				fakeComponent.PausedReturns(false)
			})

			It("tracks", func() {
				Eventually(fakeTracker.TrackCallCount).Should(Equal(1))
			})
		})
	})

	Context("when it receives shutdown signal", func() {
		JustBeforeEach(func() {
			go func() {
				process.Signal(os.Interrupt)
			}()
		})

		It("releases tracker", func() {
			Eventually(fakeTracker.ReleaseCallCount).Should(Equal(1))
		})

		It("notifies other atc it is shutting down", func() {
			Eventually(fakeNotifications.NotifyCallCount).Should(Equal(1))
		})
	})
})
