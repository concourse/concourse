package worker_test

import (
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"github.com/concourse/concourse/worker"
	"github.com/concourse/concourse/worker/baggageclaim/baggageclaimfakes"
	"github.com/concourse/concourse/worker/workerfakes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Volume Sweeper", func() {
	const (
		sweepInterval = 1 * time.Second
		maxInFlight   = uint16(1)
	)

	var (
		testLogger = lagertest.NewTestLogger("volume-sweeper")

		fakeTSAClient *workerfakes.FakeTSAClient
		fakeBcClient  *baggageclaimfakes.FakeClient

		osSignal chan os.Signal
		exited   chan struct{}
	)

	BeforeEach(func() {
		osSignal = make(chan os.Signal)
		exited = make(chan struct{})

		fakeTSAClient = new(workerfakes.FakeTSAClient)
		fakeTSAClient.ReportVolumesReturns(nil)
		fakeTSAClient.VolumesToDestroyReturns([]string{}, nil)

		fakeBcClient = new(baggageclaimfakes.FakeClient)
		fakeBcClient.ListVolumesReturns(nil, nil)
	})

	JustBeforeEach(func() {
		sweeper := worker.NewVolumeSweeper(testLogger, sweepInterval, fakeTSAClient, fakeBcClient, maxInFlight)
		go func() {
			_ = sweeper.Run(osSignal, make(chan struct{}))
			close(exited)
		}()
	})

	AfterEach(func() {
		close(osSignal)
		<-exited
	})

	It("calls CleanupOrphanedVolumes on each tick", func() {
		Eventually(fakeBcClient.CleanupOrphanedVolumesCallCount).Should(BeNumerically(">=", 1))
	})

	Context("when CleanupOrphanedVolumes returns an error", func() {
		BeforeEach(func() {
			fakeBcClient.CleanupOrphanedVolumesReturns(errors.New("cleanup-failed"))
		})

		It("logs the error but continues sweeping", func() {
			// Wait for at least 2 ticks to confirm it keeps running
			Eventually(fakeBcClient.CleanupOrphanedVolumesCallCount).Should(BeNumerically(">=", 2))
			logs := testLogger.Logs()
			Expect(len(logs)).To(BeNumerically(">=", 2))
			Expect(logs[0].LogLevel).To(Equal(lager.ERROR))
			Expect(logs[0].Data["error"]).To(Equal("cleanup-failed"))
		})
	})
})
