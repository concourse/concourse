package topgun_test

import (
	"os"
	"regexp"
	"strings"
	"time"

	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Worker landing", func() {
	landWorker := func() (string, boshInstance) {
		workerToLand := flyTable("workers")[0]["name"]

		// the bosh release ensures the first guid segment matches the first guid
		// segment of the instance ID, so that they can be correlated
		guidSegments := strings.Split(workerToLand, "-")
		prefix := guidSegments[0]

		var instance boshInstance
		for _, i := range JobInstances("worker") {
			if strings.HasPrefix(i.ID, prefix) {
				instance = i
				break
			}
		}

		Expect(instance.ID).ToNot(BeEmpty(), "should have found a corresponding bosh instance")

		// unmonitor worker, otherwise monit will just restart it once it's landed
		bosh("ssh", instance.Name, "-c", "sudo /var/vcap/bosh/bin/monit unmonitor worker")

		// land worker via fly; this will cause the worker process to exit
		fly.Run("land-worker", "-w", workerToLand)

		return workerToLand, instance
	}

	startLandedWorker := func(instance boshInstance) {
		bosh("ssh", instance.Name, "-c", "sudo /var/vcap/bosh/bin/monit monitor worker")
		bosh("ssh", instance.Name, "-c", "sudo /var/vcap/bosh/bin/monit start worker")
	}

	Context("with two workers available", func() {
		BeforeEach(func() {
			Deploy(
				"deployments/concourse.yml",
				"-o", "operations/worker-instances.yml",
				"-v", "worker_instances=2",
			)
		})

		Describe("landing the worker", func() {
			var landingWorkerName string
			var landingWorkerInstance boshInstance

			JustBeforeEach(func() {
				landingWorkerName, landingWorkerInstance = landWorker()
			})

			AfterEach(func() {
				startLandedWorker(landingWorkerInstance)
			})

			Context("while in landing or landed state", func() {
				It("is not used for new workloads", func() {
					for i := 0; i < 10; i++ {
						fly.Run("execute", "-c", "tasks/tiny.yml")
						usedWorkers := workersWithContainers()
						Expect(usedWorkers).To(HaveLen(1))
						Expect(usedWorkers).ToNot(ContainElement(landingWorkerName))
					}
				})

				It("can be pruned", func() {
					fly.Run("prune-worker", "-w", landingWorkerName)
					waitForWorkersToBeRunning(1)
				})
			})
		})
	})

	describeLandingTheWorker := func() {
		Describe("landing the worker", func() {
			var landingWorkerName string
			var landingWorkerInstance boshInstance

			JustBeforeEach(func() {
				landingWorkerName, landingWorkerInstance = landWorker()
			})

			AfterEach(func() {
				startLandedWorker(landingWorkerInstance)
			})

			Context("with volumes and containers present", func() {
				var preservedContainerID string

				BeforeEach(func() {
					By("setting pipeline that creates volumes for image")
					fly.Run("set-pipeline", "-n", "-c", "pipelines/get-task.yml", "-p", "topgun")

					By("unpausing the pipeline")
					fly.Run("unpause-pipeline", "-p", "topgun")

					By("triggering a job")
					buildSession := fly.Start("trigger-job", "-w", "-j", "topgun/simple-job")
					Eventually(buildSession).Should(gbytes.Say("fetching .*busybox.*"))
					<-buildSession.Exited
					Expect(buildSession.ExitCode()).To(Equal(0))

					By("getting identifier for check container")
					hijackSession := fly.Start("hijack", "-c", "topgun/tick-tock", "--", "hostname")
					<-hijackSession.Exited
					Expect(buildSession.ExitCode()).To(Equal(0))

					preservedContainerID = string(hijackSession.Out.Contents())
				})

				It("keeps volumes and containers after restart", func() {
					By("starting the worker back up")
					waitForLandedWorker()
					startLandedWorker(landingWorkerInstance)
					waitForWorkersToBeRunning(1)

					By("retaining cached image resource in second job build")
					buildSession := fly.Start("trigger-job", "-w", "-j", "topgun/simple-job")
					<-buildSession.Exited
					Expect(buildSession).NotTo(gbytes.Say("fetching .*busybox.*"))
					Expect(buildSession.ExitCode()).To(Equal(0))

					By("retaining check containers")
					hijackSession := fly.Start("hijack", "-c", "topgun/tick-tock", "--", "hostname")
					<-hijackSession.Exited
					Expect(buildSession.ExitCode()).To(Equal(0))

					currentContainerID := string(hijackSession.Out.Contents())
					Expect(currentContainerID).To(Equal(preservedContainerID))
				})
			})

			Context("with an interruptible build in-flight", func() {
				var buildSession *gexec.Session

				BeforeEach(func() {
					By("setting pipeline that has an infinite but interruptible job")
					fly.Run("set-pipeline", "-n", "-c", "pipelines/interruptible.yml", "-p", "topgun")

					By("unpausing the pipeline")
					fly.Run("unpause-pipeline", "-p", "topgun")

					By("triggering a job")
					buildSession = fly.Start("trigger-job", "-w", "-j", "topgun/interruptible-job")
					Eventually(buildSession).Should(gbytes.Say("waiting forever"))
				})

				It("does not wait for the build", func() {
					By("landing without the drain timeout kicking in")
					waitForLandedWorker()
				})
			})

			Context("with uninterruptible build in-flight", func() {
				var buildSession *gexec.Session
				var buildID string

				BeforeEach(func() {
					buildSession = fly.Start("execute", "-c", "tasks/wait.yml")
					Eventually(buildSession).Should(gbytes.Say("executing build"))

					buildRegex := regexp.MustCompile(`executing build (\d+)`)
					matches := buildRegex.FindSubmatch(buildSession.Out.Contents())
					buildID = string(matches[1])

					Eventually(buildSession).Should(gbytes.Say("waiting for /tmp/stop-waiting"))
				})

				AfterEach(func() {
					buildSession.Signal(os.Interrupt)
					<-buildSession.Exited
				})

				It("waits for the build", func() {
					Consistently(func() string {
						return workerState(landingWorkerName)
					}, 5*time.Minute).Should(Equal("landing"))
				})

				It("finishes landing once the build is done", func() {
					By("hijacking the build to tell it to finish")
					fly.Run("hijack", "-b", buildID, "-s", "one-off", "--", "touch", "/tmp/stop-waiting")

					By("waiting for the build to exit")
					Eventually(buildSession).Should(gbytes.Say("done"))
					<-buildSession.Exited
					Expect(buildSession.ExitCode()).To(Equal(0))

					By("successfully landing")
					waitForLandedWorker()
				})
			})
		})
	}

	Context("with one worker", func() {
		BeforeEach(func() {
			Deploy("deployments/concourse.yml")
			waitForRunningWorker()
		})

		describeLandingTheWorker()
	})

	Context("with a single team worker", func() {
		BeforeEach(func() {
			Deploy(
				"deployments/concourse.yml",
				"-o", "operations/worker-instances.yml",
				"-v", "worker_instances=0",
			)

			fly.Run("set-team", "--non-interactive", "-n", "team-a", "--local-user", atcUsername)

			Deploy(
				"deployments/concourse.yml",
				"-o", "operations/worker-team.yml",
			)

			fly.Run("login", "-c", atcExternalURL, "-n", "team-a", "-u", atcUsername, "-p", atcPassword)

			// wait for the team's worker to arrive now that team exists
			waitForRunningWorker()
		})

		describeLandingTheWorker()
	})
})
