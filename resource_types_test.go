package topgun_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("A pipeline-provided resource type", func() {
	BeforeEach(func() {
		Deploy("deployments/single-vm-no-gc.yml")
	})

	AfterEach(func() {
		// restore deployment to drainable state
		deploy := StartDeploy("deployments/single-vm.yml")

		var workers []string
		Eventually(func() []string {
			workers = workersBy("state", "landing")
			return workers
		}).Should(HaveLen(1))

		fly("prune-worker", "-w", workers[0])

		<-deploy.Exited
		Expect(deploy.ExitCode()).To(Equal(0))
	})

	It("does not result in redundant containers when running resource actions", func() {
		By("setting a pipeline")
		fly("set-pipeline", "-n", "-c", "pipelines/custom-types.yml", "-p", "pipe")
		fly("unpause-pipeline", "-p", "pipe")

		By("triggering the build")
		buildSession := spawnFly("trigger-job", "-w", "-j", "pipe/get-10m")
		<-buildSession.Exited
		Expect(buildSession.ExitCode()).To(Equal(1))

		By("expecting a container for the resource check, resource type check, and task image check")
		Expect(containersBy("type", "check")).To(HaveLen(3))

		By("expecting a container for the resource check, resource type check, build resource image get, build get, build task image check, build task image get, and build task")
		expectedContainersBefore := 7
		Expect(flyTable("containers")).Should(HaveLen(expectedContainersBefore))

		By("triggering the build again")
		buildSession = spawnFly("trigger-job", "-w", "-j", "pipe/get-10m")
		<-buildSession.Exited
		Expect(buildSession.ExitCode()).To(Equal(1))

		By("expecting only one additional check container for the task's image check")
		Expect(containersBy("type", "check")).To(HaveLen(4))

		By("expecting to only have new containers for build task image check and build task")
		Expect(flyTable("containers")).Should(HaveLen(expectedContainersBefore + 2))
	})
})
