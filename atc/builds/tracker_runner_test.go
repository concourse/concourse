package builds_test

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"

	. "github.com/concourse/concourse/v5/atc/builds"
	"github.com/concourse/concourse/v5/atc/builds/buildsfakes"
)

var _ = Describe("TrackerRunner", func() {
	var fakeTracker *buildsfakes.FakeBuildTracker
	var fakeNotifications *buildsfakes.FakeNotifications
	var fakeClock *fakeclock.FakeClock
	var tracked <-chan struct{}
	var shutdownNotify chan bool
	var buildStartedNotify chan bool
	var trackerRunner TrackerRunner
	var process ifrit.Process
	var interval = 10 * time.Second

	var logger *lagertest.TestLogger

	BeforeEach(func() {
		fakeTracker = new(buildsfakes.FakeBuildTracker)
		fakeNotifications = new(buildsfakes.FakeNotifications)

		t := make(chan struct{})
		tracked = t
		fakeTracker.TrackStub = func() {
			t <- struct{}{}
		}

		logger = lagertest.NewTestLogger("test")

		shutdownNotify = make(chan bool)
		buildStartedNotify = make(chan bool)

		fakeNotifications.ListenReturnsOnCall(0, shutdownNotify, nil)
		fakeNotifications.ListenReturnsOnCall(1, buildStartedNotify, nil)

		fakeClock = fakeclock.NewFakeClock(time.Unix(0, 123))

		trackerRunner = TrackerRunner{
			Tracker:       fakeTracker,
			Notifications: fakeNotifications,
			Interval:      interval,
			Clock:         fakeClock,
			Logger:        logger,
		}
	})

	JustBeforeEach(func() {
		process = ifrit.Invoke(trackerRunner)
	})

	AfterEach(func() {
		process.Signal(os.Interrupt)
		<-process.Wait()
	})

	It("tracks immediately", func() {
		<-tracked
	})

	Context("when it receives an ATC shutdown notice", func() {
		JustBeforeEach(func() {
			<-tracked
			go func() {
				shutdownNotify <- true
			}()
		})

		It("tracks", func() {
			By("waiting for it to track again")
			<-tracked
			By("consistently not tracking again")
			Consistently(tracked).ShouldNot(Receive())
		})
	})

	Context("when it receives a build started notice", func() {
		JustBeforeEach(func() {
			<-tracked
			go func() {
				buildStartedNotify <- true
			}()
		})

		It("tracks", func() {
			By("waiting for it to track again")
			<-tracked
			By("consistently not tracking again")
			Consistently(tracked).ShouldNot(Receive())
		})
	})

	Context("when it receives shutdown signal", func() {
		JustBeforeEach(func() {
			<-tracked
			go func() {
				process.Signal(os.Interrupt)
			}()
		})

		It("releases tracker", func() {
			<-process.Wait()
			Expect(fakeTracker.ReleaseCallCount()).To(Equal(1))
		})

		It("notifies other atc it is shutting down", func() {
			<-process.Wait()
			Expect(fakeNotifications.NotifyCallCount()).To(Equal(1))
		})
	})

	Context("when the interval elapses", func() {
		JustBeforeEach(func() {
			<-tracked
			fakeClock.Increment(interval)
		})

		It("tracks", func() {
			<-tracked
			Consistently(tracked).ShouldNot(Receive())
		})

		Context("when the interval elapses", func() {
			JustBeforeEach(func() {
				<-tracked
				fakeClock.Increment(interval)
			})

			It("tracks again", func() {
				<-tracked
				Consistently(tracked).ShouldNot(Receive())
			})
		})
	})
})
