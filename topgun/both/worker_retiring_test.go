package topgun_test

import (
	. "github.com/concourse/concourse/topgun/common"
	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Worker retiring", func() {
	BeforeEach(func() {
		Deploy("deployments/concourse.yml")
	})

	It("deletes all containers and volumes when worker is gone", func() {
		By("setting pipeline that creates resource cache")
		Fly.Run("set-pipeline", "-n", "-c", "pipelines/get-task.yml", "-p", "worker-retiring-test")

		By("unpausing the pipeline")
		Fly.Run("unpause-pipeline", "-p", "worker-retiring-test")

		By("checking resource")
		Fly.Run("check-resource", "-r", "worker-retiring-test/tick-tock")

		By("getting the worker containers")
		containersBefore := FlyTable("containers")
		Expect(containersBefore).To(HaveLen(1))

		By("getting the worker volumes")
		volumesBefore := FlyTable("volumes")
		Expect(volumesBefore).ToNot(BeEmpty())

		By("retiring the worker")
		Deploy("deployments/concourse.yml", "-o", "operations/retire-worker.yml")

		By("getting the worker containers")
		containersAfter := FlyTable("containers")
		Expect(containersAfter).To(HaveLen(0))

		By("getting the worker volumes")
		volumesAfter := FlyTable("volumes")
		Expect(volumesAfter).To(HaveLen(0))
	})
})
