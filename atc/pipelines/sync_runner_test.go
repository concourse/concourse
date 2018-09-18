package pipelines_test

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	. "github.com/concourse/atc/pipelines"
	"github.com/concourse/atc/pipelines/pipelinesfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("Pipelines Sync Runner", func() {
	var fakeSyncer *pipelinesfakes.FakePipelineSyncer
	var synced <-chan struct{}
	var interval = 10 * time.Second
	var fakeClock *fakeclock.FakeClock
	var runner SyncRunner
	var process ifrit.Process

	BeforeEach(func() {
		fakeSyncer = new(pipelinesfakes.FakePipelineSyncer)

		s := make(chan struct{})
		synced = s
		fakeSyncer.SyncStub = func() {
			s <- struct{}{}
		}

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
		<-synced
	})

	Context("when the interval elapses", func() {
		JustBeforeEach(func() {
			<-synced
			fakeClock.Increment(interval)
		})

		It("syncs again", func() {
			<-synced
			Consistently(fakeSyncer.SyncCallCount).Should(Equal(2))
		})

		Context("when the interval elapses", func() {
			JustBeforeEach(func() {
				<-synced
				fakeClock.Increment(interval)
			})

			It("syncs again", func() {
				<-synced
				Consistently(fakeSyncer.SyncCallCount).Should(Equal(3))
			})
		})
	})
})
