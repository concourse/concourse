package topgun_test

import (
	. "github.com/concourse/concourse/topgun/common"
	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Unique Version History", func() {
	BeforeEach(func() {
		Deploy("deployments/concourse.yml",
			"-o", "operations/enable-global-resources.yml")
		_ = WaitForRunningWorker()
	})

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
