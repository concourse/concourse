package builds_test

import (
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
)

var _ = Describe("TrackerRunner", func() {
	var (
		trackerRunner TrackerRunner
		process       ifrit.Process

		logger               *lagertest.TestLogger
		fakeTracker          *buildsfakes.FakeBuildTracker
		fakeComponent        *dbfakes.FakeComponent
		fakeComponentFactory *dbfakes.FakeComponentFactory
		fakeNotifications    *buildsfakes.FakeNotifications
		fakeClock            *fakeclock.FakeClock

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

				It("tracks", func() {
					Eventually(fakeTracker.TrackCallCount).Should(Equal(1))
				})

				It("updates last ran", func() {
					Eventually(fakeComponent.UpdateLastRanCallCount).Should(Equal(1))
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
