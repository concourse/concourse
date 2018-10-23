package topgun_test

import (
	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Worker retiring", func() {
	var (
		deployment string
	)

	BeforeEach(func() {
		deployment = "deployments/concourse-separate-forwarded-worker.yml"
		Deploy(deployment)
		_ = waitForRunningWorker()
	})

	It("deletes all containers and volumes when worker is gone", func() {
		By("setting pipeline that creates resource cache")
		fly("set-pipeline", "-n", "-c", "pipelines/get-task.yml", "-p", "worker-retiring-test")

		By("unpausing the pipeline")
		fly("unpause-pipeline", "-p", "worker-retiring-test")

		By("checking resource")
		fly("check-resource", "-r", "worker-retiring-test/tick-tock")

		By("getting the worker containers")
		containersBefore := flyTable("containers")
		Expect(containersBefore).To(HaveLen(1))

		By("getting the worker volumes")
		volumesBefore := flyTable("volumes")
		Expect(volumesBefore).ToNot(BeEmpty())

		By("retiring the worker")
		Deploy(deployment, "-o", "operations/retire-worker.yml")

		By("getting the worker containers")
		containersAfter := flyTable("containers")
		Expect(containersAfter).To(HaveLen(0))

		By("getting the worker volumes")
		volumesAfter := flyTable("volumes")
		Expect(volumesAfter).To(HaveLen(0))
	})
})
