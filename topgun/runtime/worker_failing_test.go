package topgun_test

import (
	. "github.com/concourse/concourse/topgun/common"
	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Worker failing", func() {
	BeforeEach(func() {
		Deploy(
			"deployments/concourse.yml",
			"-o", "operations/add-other-worker.yml",
			"-o", "operations/other-worker-doomed.yml",
			"-o", "operations/fast-gc.yml",
		)
	})

	Context("when the worker becomes unresponsive", func() {
		BeforeEach(func() {
			By("setting a pipeline that uses the doomed worker")
			Fly.Run("set-pipeline", "-n", "-c", "pipelines/controlled-trigger-doomed-worker.yml", "-p", "worker-failing-test")
			Fly.Run("unpause-pipeline", "-p", "worker-failing-test")

			By("running the build on the doomed worker")
			Fly.Run("trigger-job", "-w", "-j", "worker-failing-test/use-doomed-worker")

			By("making baggageclaim become unresponsive on the doomed worker")
			Bosh("ssh", "other_worker/0", "-c", "sudo pkill -F /var/vcap/sys/run/worker/worker.pid -STOP")

			By("discovering a new version to force the existing volume to be no longer desired")
			Fly.Run("check-resource", "-r", "worker-failing-test/controlled-trigger", "-f", "version:second")
		})

		AfterEach(func() {
			Bosh("ssh", "other_worker/0", "-c", "sudo pkill -F /var/vcap/sys/run/worker/worker.pid -CONT")
			WaitForWorkersToBeRunning(2)
		})

		It("puts the worker in stalled state and does not lock up garbage collection", func() {
			By("waiting for the doomed worker to stall")
			Eventually(WaitForStalledWorker()).ShouldNot(BeEmpty())

			By("running the build on the safe worker")
			Fly.Run("trigger-job", "-w", "-j", "worker-failing-test/use-safe-worker")

			By("having a cache for the controlled-trigger resource")
			Expect(VolumesByResourceType("mock")).ToNot(BeEmpty())

			By("discovering a new version force the existing volume on the safe worker to be no longer desired")
			Fly.Run("check-resource", "-r", "worker-failing-test/controlled-trigger", "-f", "version:third")

			By("eventually garbage collecting the volume from the safe worker")
			Eventually(func() []string {
				return VolumesByResourceType("mock")
			}).Should(BeEmpty())
		})
	})
})
