package topgun_test

import (
	"bytes"
	"os"
	"regexp"
	"time"

	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("[#129726011] Worker landing", func() {
	Context("with two workers available", func() {
		BeforeEach(func() {
			Deploy("deployments/two-forwarded-workers.yml")
		})

		Describe("restarting the worker", func() {
			var restartingWorkerName string
			var restartSession *gexec.Session

			JustBeforeEach(func() {
				restartSession = spawnBosh("restart", "worker/0")
				restartingWorkerName = waitForLandingOrLandedWorker()
			})

			AfterEach(func() {
				<-restartSession.Exited
			})

			Context("while in landing or landed state", func() {
				// technically this is timing-dependent but it doesn't seem worth the
				// time cost of explicit tests for both

				It("is not used for new workloads", func() {
					for i := 0; i < 10; i++ {
						fly("execute", "-c", "tasks/tiny.yml")
						usedWorkers := workersWithContainers()
						Expect(usedWorkers).To(HaveLen(1))
						Expect(usedWorkers).ToNot(ContainElement(restartingWorkerName))
					}
				})

				It("can be pruned", func() {
					fly("prune-worker", "-w", restartingWorkerName)
					waitForWorkersToBeRunning()
				})
			})

			Context("with a build in-flight", func() {
				var buildSession *gexec.Session
				var buildID string

				BeforeEach(func() {
					buildSession = spawnFly("execute", "-c", "tasks/wait.yml")
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
					Eventually(restartSession).Should(gbytes.Say(`Updating (instance|job)`))
					Consistently(restartSession, 5*time.Minute).ShouldNot(gexec.Exit())
				})

				It("finishes restarting once the build is done", func() {
					By("hijacking the build to tell it to finish")
					<-spawnFly(
						"hijack",
						"-b", buildID,
						"-s", "one-off",
						"touch", "/tmp/stop-waiting",
					).Exited

					By("waiting for the build to exit")
					Eventually(buildSession).Should(gbytes.Say("done"))
					<-buildSession.Exited
					Expect(buildSession.ExitCode()).To(Equal(0))

					By("successfully restarting")
					<-restartSession.Exited
					Expect(restartSession.ExitCode()).To(Equal(0))
				})
			})
		})

		// Describe("recreating the worker", func() {
		// 	var landingWorkerName string
		// 	var recreateSession *gexec.Session

		// 	JustBeforeEach(func() {
		// 		recreateSession = spawnBosh("recreate", "worker/0")
		// 		landingWorkerName = waitForLandingWorker()
		// 	})

		// 	Describe("after the recreate is complete", func() {
		// 		XIt("no longer has the volumes", func() {
		// 		})
		// 	})
		// })
	})

	describeRestartingTheWorker := func() {
		Describe("restarting the worker", func() {
			var restartSession *gexec.Session

			JustBeforeEach(func() {
				restartSession = spawnBosh("restart", "worker/0")
			})

			Context("with volumes and containers present", func() {
				var preservedContainerID string

				BeforeEach(func() {
					By("setting pipeline that creates volumes for image")
					fly("set-pipeline", "-n", "-c", "pipelines/get-task.yml", "-p", "topgun")

					By("unpausing the pipeline")
					fly("unpause-pipeline", "-p", "topgun")

					By("triggering a job")
					buildSession := spawnFly("trigger-job", "-w", "-j", "topgun/simple-job")
					Eventually(buildSession).Should(gbytes.Say("Pulling .*busybox.*"))
					<-buildSession.Exited
					Expect(buildSession.ExitCode()).To(Equal(0))

					By("getting identifier for check container")
					hijackSession := spawnFly("hijack", "-c", "topgun/tick-tock", "--", "hostname")
					<-hijackSession.Exited
					Expect(buildSession.ExitCode()).To(Equal(0))

					preservedContainerID = string(hijackSession.Out.Contents())
				})

				It("keeps volumes and containers after restart", func() {
					By("completing the restart")
					<-restartSession.Exited
					Expect(restartSession.ExitCode()).To(Equal(0))

					By("retaining cached image resource in second job build")
					buildSession := spawnFly("trigger-job", "-w", "-j", "topgun/simple-job")
					<-buildSession.Exited
					Expect(buildSession).NotTo(gbytes.Say("Pulling .*busybox.*"))
					Expect(buildSession.ExitCode()).To(Equal(0))

					By("retaining check containers")
					hijackSession := spawnFly("hijack", "-c", "topgun/tick-tock", "--", "hostname")
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
					fly("set-pipeline", "-n", "-c", "pipelines/interruptible.yml", "-p", "topgun")

					By("unpausing the pipeline")
					fly("unpause-pipeline", "-p", "topgun")

					By("triggering a job")
					buildSession = spawnFly("trigger-job", "-w", "-j", "topgun/interruptible-job")
					Eventually(buildSession).Should(gbytes.Say("waiting forever"))
				})

				It("does not wait for the build", func() {
					By("completing the restart without the drain timeout kicking in")
					Eventually(restartSession, 5*time.Minute).Should(gexec.Exit(0))
				})
			})
		})
	}

	Context("with one worker", func() {
		BeforeEach(func() {
			Deploy("deployments/one-forwarded-worker.yml")
		})

		describeRestartingTheWorker()
	})

	Context("with a single team worker", func() {
		BeforeEach(func() {
			Deploy("deployments/team-worker.yml")

			setTeam := spawnFlyInteractive(bytes.NewBufferString("y\n"), "set-team", "-n", "team-a", "--no-really-i-dont-want-any-auth")
			<-setTeam.Exited
			Expect(setTeam.ExitCode()).To(Equal(0))

			fly("login", "-c", atcExternalURL, "-n", "team-a")

			// wait for the team's worker to arrive now that team exists
			waitForRunningWorker()
		})

		describeRestartingTheWorker()
	})
})
