package topgun_test

import (
	"time"

	. "github.com/concourse/concourse/topgun/common"
	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Hijacked containers", func() {
	BeforeEach(func() {
		Deploy("deployments/concourse.yml")
	})

	getContainer := func(condition, value string) func() hijackedContainerResult {
		return func() (h hijackedContainerResult) {
			containers := FlyTable("containers")

			var containerHandle string
			for _, c := range containers {
				if c[condition] == value {
					containerHandle = c["handle"]
					h.flyContainerExists = true

					break
				}
			}

			_, err := WorkerGardenClient.Lookup(containerHandle)
			if err == nil {
				h.gardenContainerExists = true
			}

			return
		}
	}

	It("does not delete hijacked build containers from the database, and sets a 5 minute TTL on the container in garden", func() {
		By("setting the pipeline that has a build")
		Fly.Run("set-pipeline", "-n", "-c", "pipelines/task-waiting.yml", "-p", "hijacked-containers-test")

		By("triggering the build")
		Fly.Run("unpause-pipeline", "-p", "hijacked-containers-test")
		buildSession := Fly.Start("trigger-job", "-w", "-j", "hijacked-containers-test/simple-job")
		//For the initializing block
		Eventually(buildSession).Should(gbytes.Say("echo 'waiting for /tmp/stop-waiting to exist'"))
		//For the output from the running step
		Eventually(buildSession).Should(gbytes.Say("waiting for /tmp/stop-waiting to exist"))

		By("hijacking into the build container")
		hijackSession := Fly.Start(
			"hijack",
			"-j", "hijacked-containers-test/simple-job",
			"-b", "1",
			"-s", "simple-task",
			"sleep", "120",
		)

		By("finishing the build")
		Eventually(func()int {
			hS := Fly.Start(
				"hijack",
				"-j", "hijacked-containers-test/simple-job",
				"-s", "simple-task",
				"touch", "/tmp/stop-waiting",
			)
			<-hS.Exited
			return hS.ExitCode()
		}).Should(Equal(0))

		<-buildSession.Exited

		By("triggering a new build")
		buildSession = Fly.Start("trigger-job", "-w", "-j", "hijacked-containers-test/simple-job")
		//For the initializing block
		Eventually(buildSession).Should(gbytes.Say("echo 'waiting for /tmp/stop-waiting to exist'"))
		//For the output from the running step
		Eventually(buildSession).Should(gbytes.Say("waiting for /tmp/stop-waiting to exist"))

		Eventually(func()int {
			hS := Fly.Start(
				"hijack",
				"-j", "hijacked-containers-test/simple-job",
				"-b", "2",
				"-s", "simple-task",
				"touch", "/tmp/stop-waiting",
			)
			<-hS.Exited
			return hS.ExitCode()
		}).Should(Equal(0))
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
		buildSession := Fly.Start("execute", "-c", "tasks/wait.yml")
		//For the initializing block
		Eventually(buildSession).Should(gbytes.Say("echo 'waiting for /tmp/stop-waiting to exist'"))
		//For the output from the running step
		Eventually(buildSession).Should(gbytes.Say("waiting for /tmp/stop-waiting to exist"))

		By("hijacking into the build container")
		hijackSession := Fly.Start(
			"hijack",
			"-b", "1",
			"--",
			"while true; do sleep 1; done",
		)

		By("waiting for build to finish")
		Eventually(func()int {
			hS := Fly.Start(
				"hijack",
				"-b", "1",
				"touch", "/tmp/stop-waiting",
			)
			<-hS.Exited
			return hS.ExitCode()
		}).Should(Equal(0))
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
		Fly.Run("set-pipeline", "-n", "-c", "pipelines/get-task.yml", "-p", "hijacked-resource-test")
		Fly.Run("unpause-pipeline", "-p", "hijacked-resource-test")

		By("checking resource")
		Fly.Run("check-resource", "-r", "hijacked-resource-test/tick-tock")

		containers := FlyTable("containers")
		var checkContainerHandle string
		for _, c := range containers {
			if c["type"] == "check" {
				checkContainerHandle = c["handle"]
				break
			}
		}
		Expect(checkContainerHandle).ToNot(BeEmpty())

		By("hijacking into the resource container")
		hijackSession := Fly.Start(
			"hijack",
			"-c", "hijacked-resource-test/tick-tock",
			"sleep", "120",
		)

		By("reconfiguring pipeline without resource")
		Fly.Run("set-pipeline", "-n", "-c", "pipelines/task-waiting.yml", "-p", "hijacked-resource-test")

		By("verifying the hijacked container exists via Garden")
		_, err := WorkerGardenClient.Lookup(checkContainerHandle)
		Expect(err).NotTo(HaveOccurred())

		By("unhijacking and seeing the container removed via fly/Garden after 5 minutes")
		hijackSession.Interrupt()
		<-hijackSession.Exited

		Eventually(getContainer("type", "check"), 10*time.Minute, 30*time.Second).Should(Equal(hijackedContainerResult{false, false}))
	})
})

type hijackedContainerResult struct {
	flyContainerExists    bool
	gardenContainerExists bool
}
