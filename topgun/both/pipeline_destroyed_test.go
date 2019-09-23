package topgun_test

import (
	"time"

	"code.cloudfoundry.org/garden"
	. "github.com/concourse/concourse/topgun/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Garbage collecting containers for destroyed pipelines", func() {
	BeforeEach(func() {
		Deploy("deployments/concourse.yml")
	})

	It("should be removed", func() {
		By("setting a pipeline")
		Fly.Run("set-pipeline", "-n", "-c", "pipelines/get-task-put.yml", "-p", "pipeline-destroyed-test")

		By("kicking off a build")
		Fly.Run("unpause-pipeline", "-p", "pipeline-destroyed-test")
		buildSession := Fly.Start("trigger-job", "-w", "-j", "pipeline-destroyed-test/simple-job")

		<-buildSession.Exited
		Expect(buildSession.ExitCode()).To(Equal(0))

		By("verifying that containers exist")
		containerTable := FlyTable("containers")
		Expect(containerTable).To(HaveLen(7))

		var (
			err error
		)
		for _, c := range containerTable {
			_, err = WorkerGardenClient.Lookup(c["handle"])
			Expect(err).NotTo(HaveOccurred())
		}

		By("destroying the pipeline")
		Fly.Run("destroy-pipeline", "-n", "-p", "pipeline-destroyed-test")

		By("verifying the containers don't exist")
		Eventually(func() int {
			return len(FlyTable("containers"))
		}, 5*time.Minute, 30*time.Second).Should(BeZero())

		Eventually(func() []garden.Container {
			containers, err := WorkerGardenClient.Containers(nil)
			Expect(err).NotTo(HaveOccurred())

			return containers
		}, 5*time.Minute, 30*time.Second).Should(BeEmpty())
	})
})
