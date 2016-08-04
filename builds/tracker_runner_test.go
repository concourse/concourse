package builds_test

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"

	. "github.com/concourse/atc/builds"
	"github.com/concourse/atc/builds/buildsfakes"
)

var _ = Describe("TrackerRunner", func() {
	var fakeTracker *buildsfakes.FakeBuildTracker
	var fakeClock *fakeclock.FakeClock
	var tracked <-chan struct{}
	var trackerRunner TrackerRunner
	var process ifrit.Process
	var interval = 10 * time.Second

	BeforeEach(func() {
		fakeTracker = new(buildsfakes.FakeBuildTracker)

		t := make(chan struct{})
		tracked = t
		fakeTracker.TrackStub = func() {
			t <- struct{}{}
		}

		fakeClock = fakeclock.NewFakeClock(time.Unix(0, 123))

		trackerRunner = TrackerRunner{
			Tracker:  fakeTracker,
			Interval: interval,
			Clock:    fakeClock,
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
