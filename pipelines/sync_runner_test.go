package pipelines_test

import (
	"os"
	"time"

	. "github.com/concourse/atc/pipelines"
	"github.com/concourse/atc/pipelines/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/clock/fakeclock"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("Pipelines Sync Runner", func() {
	var fakeSyncer *fakes.FakePipelineSyncer
	var interval = 10 * time.Second
	var fakeClock *fakeclock.FakeClock
	var runner SyncRunner
	var process ifrit.Process

	BeforeEach(func() {
		fakeSyncer = new(fakes.FakePipelineSyncer)
		fakeClock = fakeclock.NewFakeClock(time.Unix(0, 123))

		runner = SyncRunner{
			Syncer:   fakeSyncer,
			Interval: interval,
			Clock:    fakeClock,
		}
	})

	JustBeforeEach(func() {
		process = ifrit.Invoke(runner)
	})

	AfterEach(func() {
		process.Signal(os.Interrupt)
		Eventually(process.Wait()).Should(Receive())
	})

	It("syncs immediately", func() {
		Eventually(fakeSyncer.SyncCallCount).Should(Equal(1))
	})

	Context("when the interval elapses", func() {
		JustBeforeEach(func() {
			Eventually(fakeSyncer.SyncCallCount).Should(Equal(1))
			fakeClock.Increment(interval)
		})

		It("syncs again", func() {
			Eventually(fakeSyncer.SyncCallCount).Should(Equal(2))
			Consistently(fakeSyncer.SyncCallCount).Should(Equal(2))
		})

		Context("when the interval elapses", func() {
			JustBeforeEach(func() {
				Eventually(fakeSyncer.SyncCallCount).Should(Equal(2))
				fakeClock.Increment(interval)
			})

			It("syncs again", func() {
				Eventually(fakeSyncer.SyncCallCount).Should(Equal(3))
				Consistently(fakeSyncer.SyncCallCount).Should(Equal(3))
			})
		})
	})
})
