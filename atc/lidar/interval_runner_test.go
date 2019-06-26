package lidar_test

import (
	"context"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc/lidar"
	"github.com/concourse/concourse/atc/lidar/lidarfakes"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("IntervalRunner", func() {
	var (
		intervalRunner ifrit.Runner
		process        ifrit.Process

		logger            *lagertest.TestLogger
		interval          time.Duration
		notifier          chan bool
		fakeRunner        *lidarfakes.FakeRunner
		fakeNotifications *lidarfakes.FakeNotifications

		runAt     time.Time
		runTimes  chan time.Time
		fakeClock *fakeclock.FakeClock
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		interval = time.Minute
		notifier = make(chan bool)
		fakeRunner = new(lidarfakes.FakeRunner)
		fakeNotifications = new(lidarfakes.FakeNotifications)
		fakeNotifications.ListenReturns(notifier, nil)

		runAt = time.Unix(111, 111).UTC()
		runTimes = make(chan time.Time, 100)
		fakeClock = fakeclock.NewFakeClock(runAt)

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
		)
	})

	JustBeforeEach(func() {
		process = ifrit.Invoke(intervalRunner)
	})

	AfterEach(func() {
		process.Signal(os.Interrupt)
		Eventually(process.Wait()).Should(Receive())
	})

	Context("when created", func() {
		It("runs", func() {
			Expect(<-runTimes).To(Equal(runAt))
		})
	})

	Context("when the interval elapses", func() {
		It("runs again", func() {
			Expect(<-runTimes).To(Equal(runAt))

			fakeClock.WaitForWatcherAndIncrement(interval)
			Expect(<-runTimes).To(Equal(runAt.Add(interval)))
		})
	})

	Context("when it receives a notification", func() {
		It("runs again", func() {
			Expect(<-runTimes).To(Equal(runAt))

			notifier <- true
			Expect(<-runTimes).To(Equal(runAt))
		})
	})
})
