package builds_test

import (
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/clock/fakeclock"
	"github.com/tedsuo/ifrit"

	. "github.com/concourse/atc/builds"
	"github.com/concourse/atc/builds/fakes"
)

var _ = Describe("TrackerRunner", func() {
	var fakeTracker *fakes.FakeBuildTracker
	var fakeClock *fakeclock.FakeClock
	var trackerRunner TrackerRunner
	var process ifrit.Process
	var interval = 10 * time.Second

	BeforeEach(func() {
		fakeTracker = new(fakes.FakeBuildTracker)
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
		Eventually(process.Wait()).Should(Receive())
	})

	It("tracks immediately", func() {
		Eventually(fakeTracker.TrackCallCount).Should(Equal(1))
	})

	Context("when the interval elapses", func() {
		JustBeforeEach(func() {
			Eventually(fakeTracker.TrackCallCount).Should(Equal(1))
			fakeClock.Increment(interval)
		})

		It("tracks", func() {
			Eventually(fakeTracker.TrackCallCount).Should(Equal(2))
			Consistently(fakeTracker.TrackCallCount).Should(Equal(2))
		})

		Context("when the interval elapses", func() {
			JustBeforeEach(func() {
				Eventually(fakeTracker.TrackCallCount).Should(Equal(2))
				fakeClock.Increment(interval)
			})

			It("tracks again", func() {
				Eventually(fakeTracker.TrackCallCount).Should(Equal(3))
				Consistently(fakeTracker.TrackCallCount).Should(Equal(3))
			})
		})
	})
})
