package topgun_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"time"

	"code.cloudfoundry.org/garden"
	gclient "code.cloudfoundry.org/garden/client"
	gconn "code.cloudfoundry.org/garden/client/connection"
)

var _ = Describe(":life [#130495079] Garbage collecting containers for destroyed pipelines", func() {
	var (
		gClient gclient.Client
	)

	BeforeEach(func() {
		Deploy("deployments/single-vm.yml")

		gClient = gclient.New(gconn.New("tcp", fmt.Sprintf("%s:7777", atcIP)))
	})

	It("should be removed", func() {
		By("setting a pipeline")
		fly("set-pipeline", "-n", "-c", "pipelines/get-task-put.yml", "-p", "pipeline-destroyed-test")

		By("kicking off a build")
		fly("unpause-pipeline", "-p", "pipeline-destroyed-test")
		buildSession := spawnFly("trigger-job", "-w", "-j", "pipeline-destroyed-test/simple-job")

		<-buildSession.Exited
		Expect(buildSession.ExitCode()).To(Equal(0))

		By("verifying that containers exist")
		containerTable := flyTable("containers")
		Expect(containerTable).To(HaveLen(7))

		var (
			err error
		)
		for _, c := range containerTable {
			_, err = gClient.Lookup(c["handle"])
			Expect(err).NotTo(HaveOccurred())
		}

		By("destroying the pipeline")
		fly("destroy-pipeline", "-n", "-p", "pipeline-destroyed-test")

		By("verifying the containers don't exist")
		Eventually(func() int {
			return len(flyTable("containers"))
		}, 5*time.Minute, 30*time.Second).Should(BeZero())

		Eventually(func() []garden.Container {
			containers, err := gClient.Containers(nil)
			Expect(err).NotTo(HaveOccurred())

			return containers
		}, 5*time.Minute, 30*time.Second).Should(BeEmpty())
	})
})
