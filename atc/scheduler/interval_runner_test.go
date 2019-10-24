package scheduler_test

import (
	"context"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc/scheduler"
	"github.com/concourse/concourse/atc/scheduler/schedulerfakes"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("IntervalRunner", func() {
	var (
		intervalRunner ifrit.Runner
		process        ifrit.Process

		logger     *lagertest.TestLogger
		interval   time.Duration
		fakeRunner *schedulerfakes.FakeRunner

		runAt     time.Time
		runTimes  chan time.Time
		fakeClock *fakeclock.FakeClock

		noop bool
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		interval = time.Minute
		fakeRunner = new(schedulerfakes.FakeRunner)

		runAt = time.Unix(111, 111).UTC()
		runTimes = make(chan time.Time, 100)
		fakeClock = fakeclock.NewFakeClock(runAt)

		fakeRunner.RunStub = func(ctx context.Context) error {
			runTimes <- fakeClock.Now()
			return nil
		}
	})

	JustBeforeEach(func() {
		intervalRunner = scheduler.NewIntervalRunner(
			logger,
			fakeClock,
			noop,
			fakeRunner,
			interval,
		)

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

	Context("when in noop mode", func() {
		BeforeEach(func() {
			noop = true
		})

		It("does not start scheduling builds", func() {
			Consistently(fakeRunner.RunCallCount).Should(Equal(0))
		})
	})

})
