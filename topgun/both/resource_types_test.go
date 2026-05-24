package topgun_test

import (
	. "github.com/concourse/concourse/topgun/common"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A pipeline-provided resource type", func() {
	BeforeEach(func() {
		Deploy("deployments/concourse.yml", "-o", "operations/no-gc.yml")
	})

	It("does not result in redundant containers when running resource actions", func() {
		By("setting a pipeline")
		Fly.Run("set-pipeline", "-n", "-c", "pipelines/custom-types.yml", "-p", "pipe")
		Fly.Run("unpause-pipeline", "-p", "pipe")

		By("triggering the build")
		buildSession := Fly.Start("trigger-job", "-w", "-j", "pipe/get-10m")
		Eventually(buildSession).Should(gexec.Exit(1))

		By("expecting a container for the resource check and task image check. Containers for resource types are GC'd quickly.")
		Expect(ContainersBy("type", "check")).To(HaveLen(2))

		By("expecting a container for resource check and get from get step. Expecting a container for task image check and get, and task run in task step. In total 5 containers.")
		expectedContainersBefore := 5
		Expect(FlyTable("containers")).Should(HaveLen(expectedContainersBefore))

		By("triggering the build again")
		buildSession = Fly.Start("trigger-job", "-w", "-j", "pipe/get-10m")
		Eventually(buildSession).Should(gexec.Exit(1))

		By("expecting 1 additional check containers for the task's image check")
		Expect(ContainersBy("type", "check")).To(HaveLen(3))

		By("expecting to only have new containers for build task image check and build task run")
		Expect(FlyTable("containers")).Should(HaveLen(expectedContainersBefore + 2))
	})
})

var _ = Describe("Tagged resource types", func() {
	BeforeEach(func() {
		Deploy("deployments/concourse.yml", "-o", "operations/tagged-worker.yml")

		By("setting a pipeline with tagged custom types")
		Fly.Run("set-pipeline", "-n", "-c", "pipelines/tagged-custom-types.yml", "-p", "pipe")
		Fly.Run("unpause-pipeline", "-p", "pipe")
	})

	It("is able to be used with tagged workers", func() {
		By("running a check which uses the tagged custom resource")
		Eventually(Fly.Start("check-resource", "-r", "pipe/10m")).Should(gexec.Exit(0))

		By("triggering a build which uses the tagged custom resource")
		buildSession := Fly.Start("trigger-job", "-w", "-j", "pipe/get-10m")
		Eventually(buildSession).Should(gexec.Exit(0))
	})
})
