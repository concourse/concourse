package topgun_test

import (
	"time"

	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Worker failing", func() {
	BeforeEach(func() {
		Deploy("deployments/two-workers-one-doomed-fast-gc.yml")
	})

	Context("when baggageclaim becomes unresponsive", func() {
		BeforeEach(func() {
			By("setting a pipeline that uses the doomed worker")
			fly("set-pipeline", "-n", "-c", "pipelines/controlled-timer-doomed-worker.yml", "-p", "worker-failing-test")
			fly("unpause-pipeline", "-p", "worker-failing-test")

			By("running the build on the doomed worker")
			fly("trigger-job", "-w", "-j", "worker-failing-test/use-doomed-worker")

			By("making baggageclaim become unresponsive on the doomed worker")
			bosh("ssh", "doomed_worker/0", "-c", "sudo pkill -F /var/vcap/sys/run/baggageclaim/baggageclaim.pid -STOP")

			By("running check-resource to force the existing volume to be no longer desired")
			fly("check-resource", "-r", "worker-failing-test/controlled-timer")

			// at this point, GC should try to remove the volume, and begin to hang
		})

		AfterEach(func() {
			bosh("ssh", "doomed_worker/0", "-c", "sudo pkill -F /var/vcap/sys/run/baggageclaim/baggageclaim.pid -CONT")
			waitForWorkersToBeRunning()
		})

		It("puts the worker in stalled state and does not lock up garbage collection", func() {
			By("waiting for the doomed worker to stall")
			Eventually(waitForStalledWorker()).ShouldNot(BeEmpty())

			By("running the build on the safe worker")
			fly("trigger-job", "-w", "-j", "worker-failing-test/use-safe-worker")

			By("having a cache for the controlled-timer resource")
			Expect(volumesByResourceType("time")).ToNot(BeEmpty())

			By("waiting long enough for the resource to have a new version available upon check")
			time.Sleep(5 * time.Second)

			By("running check-resource to force the existing volume on the safe worker to be no longer desired")
			fly("check-resource", "-r", "worker-failing-test/controlled-timer")

			By("eventually garbage collecting the volume from the safe worker")
			Eventually(func() []string {
				return volumesByResourceType("time")
			}).Should(BeEmpty())
		})
	})
})
