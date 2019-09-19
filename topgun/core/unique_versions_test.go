package topgun_test

import (
	. "github.com/concourse/concourse/topgun/common"
	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Unique Version History", func() {
	BeforeEach(func() {
		Deploy("deployments/concourse.yml",
			"-o", "operations/enable-global-resources.yml")
		_ = WaitForRunningWorker()
	})

	Context("with a time resource", func() {
		BeforeEach(func() {
			By("setting a pipeline with a time resource")
			Fly.Run("set-pipeline", "-n", "-c", "pipelines/time-resource.yml", "-p", "time-resource-1")

			By("unpausing the pipeline")
			Fly.Run("unpause-pipeline", "-p", "time-resource-1")

			By("setting another pipeline with a time resource")
			Fly.Run("set-pipeline", "-n", "-c", "pipelines/time-resource.yml", "-p", "time-resource-2")

			By("unpausing the pipeline")
			Fly.Run("unpause-pipeline", "-p", "time-resource-2")
		})

		It("creates unique version history for each time resource", func() {
			By("running the check for the first pipeline")
			Fly.Run("check-resource", "-r", "time-resource-1/time-resource")

			By("running the check for the second pipeline")
			Fly.Run("check-resource", "-r", "time-resource-2/time-resource")

			By("getting the versions for the first time resource")
			versions1 := Fly.GetVersions("time-resource-1", "time-resource")

			By("getting the versions for the second time resource")
			versions2 := Fly.GetVersions("time-resource-2", "time-resource")

			Expect(versions1).ToNot(Equal(versions2))
		})
	})

	Context("when a resource is specified to have a unique version history from the pipeline", func() {
		BeforeEach(func() {
			By("setting a pipeline with a unique version history resource")
			Fly.Run("set-pipeline", "-n", "-c", "pipelines/custom-unique-type.yml", "-p", "unique-resource-1")

			By("unpausing the pipeline")
			Fly.Run("unpause-pipeline", "-p", "unique-resource-1")

			By("setting another pipeline with a unique version history resource")
			Fly.Run("set-pipeline", "-n", "-c", "pipelines/custom-unique-type.yml", "-p", "unique-resource-2")

			By("unpausing the pipeline")
			Fly.Run("unpause-pipeline", "-p", "unique-resource-2")
		})

		It("creates unique version history for each unique resource", func() {
			By("running the check for the first pipeline")
			Fly.Run("check-resource", "-r", "unique-resource-1/some-resource", "-f", "version:v1")

			By("running the check for the second pipeline")
			Fly.Run("check-resource", "-r", "unique-resource-2/some-resource", "-f", "version:v2")

			By("getting the versions for the first unique resource")
			versions1 := Fly.GetVersions("unique-resource-1", "some-resource")
			Expect(versions1).To(HaveLen(1))

			By("getting the versions for the second unique resource")
			versions2 := Fly.GetVersions("unique-resource-2", "some-resource")
			Expect(versions2).To(HaveLen(1))

			Expect(versions1[0].Version).ToNot(Equal(versions2[0].Version))
		})
	})
})
