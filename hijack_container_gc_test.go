package topgun_test

import (
	"bytes"
	"fmt"
	"time"

	gclient "code.cloudfoundry.org/garden/client"
	gconn "code.cloudfoundry.org/garden/client/connection"
	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("[#129726125] Hijacked containers", func() {
	var (
		gClient gclient.Client

		containerHandle string
	)

	BeforeEach(func() {
		Deploy("deployments/single-vm.yml")

		gClient = gclient.New(gconn.New("tcp", fmt.Sprintf("%s:7777", atcIP)))
	})

	getContainer := func() (h hijackedContainerResult) {
		containers := flyTable("containers")

		for _, c := range containers {
			if c["build #"] == "1" {
				containerHandle = c["handle"]
				h.flyContainerExists = true

				break
			}
		}

		_, err := gClient.Lookup(containerHandle)
		if err == nil {
			h.gardenContainerExists = true
		}

		return
	}

	It("does not delete hijacked build containers from the database, and sets a 5 minute TTL on the container in garden", func() {
		By("setting the pipeline that has a build")
		fly("set-pipeline", "-n", "-c", "pipelines/task-waiting.yml", "-p", "hijacked-containers-test")

		By("triggering the build")
		fly("unpause-pipeline", "-p", "hijacked-containers-test")
		buildSession := spawnFly("trigger-job", "-w", "-j", "hijacked-containers-test/simple-job")
		Eventually(buildSession).Should(gbytes.Say("waiting for /tmp/stop-waiting"))

		By("hijacking into the build container")
		hijackSession := spawnFlyInteractive(
			bytes.NewBufferString("3\n"),
			"hijack",
			"-j", "hijacked-containers-test/simple-job",
			"-b", "1",
			"-s", "simple-task",
			"touch", "/tmp/stop-waiting",
			"and", "sleep", "3600",
		)

		By("triggering a new build")
		<-buildSession.Exited
		buildSession = spawnFly("trigger-job", "-w", "-j", "hijacked-containers-test/simple-job")
		Eventually(buildSession).Should(gbytes.Say("waiting for /tmp/stop-waiting"))
		<-spawnFlyInteractive(
			bytes.NewBufferString("3\n"),
			"hijack",
			"-j", "hijacked-containers-test/simple-job",
			"-b", "2",
			"-s", "simple-task",
			"touch", "/tmp/stop-waiting",
		).Exited

		By("verifying the hijacked container exists via fly and Garden")
		Consistently(getContainer, 2*time.Minute, 30*time.Second).Should(Equal(hijackedContainerResult{true, true}))

		By("unhijacking and seeing the container removed via fly/Garden after 5 minutes")
		hijackSession.Terminate()
		<-hijackSession.Exited

		Eventually(getContainer, 10*time.Minute, 30*time.Second).Should(Equal(hijackedContainerResult{false, false}))
	})

	It("does not delete hijacked one-off build containers from the database, and sets a 5 minute TTL on the container in garden", func() {
		By("triggering a one-off build")
		buildSession := spawnFly("execute", "-c", "tasks/wait.yml")
		Eventually(buildSession).Should(gbytes.Say("waiting for /tmp/stop-waiting"))

		By("hijacking into the build container")
		hijackSession := spawnFlyInteractive(
			bytes.NewBufferString("3\n"),
			"hijack",
			"-b", "1",
			"touch", "/tmp/stop-waiting",
			"and", "sleep", "3600",
		)

		By("waiting for build to finish")
		<-buildSession.Exited

		By("verifying the hijacked container exists via fly and Garden")
		Consistently(getContainer, 2*time.Minute, 30*time.Second).Should(Equal(hijackedContainerResult{true, true}))

		By("unhijacking and seeing the container removed via fly/Garden after 5 minutes")
		hijackSession.Terminate()
		<-hijackSession.Exited

		Eventually(getContainer, 10*time.Minute, 30*time.Second).Should(Equal(hijackedContainerResult{false, false}))
	})
})

type hijackedContainerResult struct {
	flyContainerExists    bool
	gardenContainerExists bool
}
