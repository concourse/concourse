package builds_test

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"

	. "github.com/concourse/atc/builds"
	"github.com/concourse/atc/builds/buildsfakes"
)

var _ = Describe("TrackerRunner", func() {
	var fakeTracker *buildsfakes.FakeBuildTracker
	var fakeListener *buildsfakes.FakeATCListener
	var fakeClock *fakeclock.FakeClock
	var tracked <-chan struct{}
	var notify chan<- bool
	var notifyrec <-chan bool
	var trackerRunner TrackerRunner
	var process ifrit.Process
	var interval = 10 * time.Second

	var logger *lagertest.TestLogger

	BeforeEach(func() {
		fakeTracker = new(buildsfakes.FakeBuildTracker)
		fakeListener = new(buildsfakes.FakeATCListener)

		t := make(chan struct{})
		tracked = t
		fakeTracker.TrackStub = func() {
			t <- struct{}{}
		}

		logger = lagertest.NewTestLogger("test")

		n := make(chan bool)
		notify = n
		notifyrec = n
		fakeListener.ListenReturns(n, nil)

		fakeClock = fakeclock.NewFakeClock(time.Unix(0, 123))

		trackerRunner = TrackerRunner{
			Tracker:   fakeTracker,
			ListenBus: fakeListener,
			Interval:  interval,
			Clock:     fakeClock,
			Logger:    logger,
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

	Context("when it recives an ATC shutdown notice", func() {
		JustBeforeEach(func() {
			<-tracked
			go func() {
				notify <- true
			}()
		})

		It("tracks", func() {
			By("waiting for it to track again")
			<-tracked
			By("consistently not tracking again")
			Consistently(tracked).ShouldNot(Receive())
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
