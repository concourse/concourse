package topgun_test

import (
	"fmt"
	"time"

	gclient "code.cloudfoundry.org/garden/client"
	gconn "code.cloudfoundry.org/garden/client/connection"
	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe(":life [#129726125] Hijacked containers", func() {
	var (
		gClient gclient.Client

		containerHandle string
	)

	BeforeEach(func() {
		Deploy("deployments/single-vm.yml")

		gClient = gclient.New(gconn.New("tcp", fmt.Sprintf("%s:7777", atcIP)))
	})

	getContainer := func(condition, value string) func() hijackedContainerResult {
		return func() (h hijackedContainerResult) {
			containers := flyTable("containers")

			for _, c := range containers {
				if c[condition] == value {
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
	}

	It("does not delete hijacked build containers from the database, and sets a 5 minute TTL on the container in garden", func() {
		By("setting the pipeline that has a build")
		fly("set-pipeline", "-n", "-c", "pipelines/task-waiting.yml", "-p", "hijacked-containers-test")

		By("triggering the build")
		fly("unpause-pipeline", "-p", "hijacked-containers-test")
		buildSession := spawnFly("trigger-job", "-w", "-j", "hijacked-containers-test/simple-job")
		Eventually(buildSession).Should(gbytes.Say("waiting for /tmp/stop-waiting"))

		By("hijacking into the build container")
		hijackSession := spawnFly(
			"hijack",
			"-j", "hijacked-containers-test/simple-job",
			"-b", "1",
			"-s", "simple-task",
			"sleep", "120",
		)

		By("finishing the build")
		<-spawnFly(
			"hijack",
			"-j", "hijacked-containers-test/simple-job",
			"-b", "1",
			"-s", "simple-task",
			"touch", "/tmp/stop-waiting",
		).Exited
		<-buildSession.Exited

		By("triggering a new build")
		buildSession = spawnFly("trigger-job", "-w", "-j", "hijacked-containers-test/simple-job")
		Eventually(buildSession).Should(gbytes.Say("waiting for /tmp/stop-waiting"))
		<-spawnFly(
			"hijack",
			"-j", "hijacked-containers-test/simple-job",
			"-b", "2",
			"-s", "simple-task",
			"touch", "/tmp/stop-waiting",
		).Exited
		<-buildSession.Exited

		By("verifying the hijacked container exists via fly and Garden")
		Consistently(getContainer("build #", "1"), 2*time.Minute, 30*time.Second).Should(Equal(hijackedContainerResult{true, true}))

		By("unhijacking and seeing the container removed via fly/Garden after 5 minutes")
		hijackSession.Interrupt()
		<-hijackSession.Exited

		Eventually(getContainer("build #", "1"), 10*time.Minute, 30*time.Second).Should(Equal(hijackedContainerResult{false, false}))
	})

	It("does not delete hijacked one-off build containers from the database, and sets a 5 minute TTL on the container in garden", func() {
		By("triggering a one-off build")
		buildSession := spawnFly("execute", "-c", "tasks/wait.yml")
		Eventually(buildSession).Should(gbytes.Say("waiting for /tmp/stop-waiting"))

		By("hijacking into the build container")
		hijackSession := spawnFly(
			"hijack",
			"-b", "1",
			"--",
			"while true; do sleep 1; done",
		)

		By("waiting for build to finish")
		<-spawnFly(
			"hijack",
			"-b", "1",
			"touch", "/tmp/stop-waiting",
		).Exited
		<-buildSession.Exited

		By("verifying the hijacked container exists via fly and Garden")
		Consistently(getContainer("build #", "1"), 2*time.Minute, 30*time.Second).Should(Equal(hijackedContainerResult{true, true}))

		By("unhijacking and seeing the container removed via fly/Garden after 5 minutes")
		hijackSession.Interrupt()
		<-hijackSession.Exited

		Eventually(getContainer("build #", "1"), 10*time.Minute, 30*time.Second).Should(Equal(hijackedContainerResult{false, false}))
	})

	It("does not delete hijacked resource containers from the database, and sets a 5 minute TTL on the container in garden", func() {
		By("setting the pipeline that has a build")
		fly("set-pipeline", "-n", "-c", "pipelines/get-task.yml", "-p", "hijacked-resource-test")
		fly("unpause-pipeline", "-p", "hijacked-resource-test")

		By("checking resource")
		fly("check-resource", "-r", "hijacked-resource-test/tick-tock")

		By("hijacking into the resource container")
		hijackSession := spawnFly(
			"hijack",
			"-c", "hijacked-resource-test/tick-tock",
			"sleep", "120",
		)

		By("reconfiguring pipeline without resource")
		fly("set-pipeline", "-n", "-c", "pipelines/task-waiting.yml", "-p", "hijacked-resource-test")

		By("verifying the hijacked container exists via fly and Garden")
		Consistently(getContainer("type", "check"), 2*time.Minute, 30*time.Second).Should(Equal(hijackedContainerResult{true, true}))

		By("unhijacking and seeing the container removed via fly/Garden after 5 minutes")
		hijackSession.Interrupt()
		<-hijackSession.Exited

		Eventually(getContainer("type", "check"), 40*time.Minute, 30*time.Second).Should(Equal(hijackedContainerResult{false, false}))
	})
})

type hijackedContainerResult struct {
	flyContainerExists    bool
	gardenContainerExists bool
}
