package topgun_test

import (
	"os"
	"regexp"
	"time"

	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("[#129726011] Worker stalling", func() {
	Context("with two workers available", func() {
		BeforeEach(func() {
			Deploy("deployments/concourse-separate-forwarded-worker.yml", "-o", "operations/separate-worker-two.yml")
		})

		It("initially runs tasks across all workers", func() {
			usedWorkers := map[string]struct{}{}
			Eventually(func() map[string]struct{} {
				fly("execute", "-c", "tasks/tiny.yml")
				workerNames := workersWithContainers()
				for _, w := range workerNames {
					usedWorkers[w] = struct{}{}
				}
				return usedWorkers
			}, 10*time.Minute).Should(HaveLen(2))
		})

		Context("when one worker goes away", func() {
			var stalledWorkerName string

			BeforeEach(func() {
				bosh("ssh", "worker/0", "-c", "sudo /var/vcap/bosh/bin/monit stop worker")
				bosh("ssh", "worker/0", "-c", "sudo /var/vcap/bosh/bin/monit stop garden")
				stalledWorkerName = waitForStalledWorker()
			})

			AfterEach(func() {
				bosh("ssh", "worker/0", "-c", "sudo /var/vcap/bosh/bin/monit start worker")
				bosh("ssh", "worker/0", "-c", "sudo /var/vcap/bosh/bin/monit start garden")
				waitForWorkersToBeRunning()
			})

			It("enters 'stalled' state and is no longer used for new containers", func() {
				for i := 0; i < 10; i++ {
					fly("execute", "-c", "tasks/tiny.yml")
					usedWorkers := workersWithContainers()
					Expect(usedWorkers).To(HaveLen(1))
					Expect(usedWorkers).ToNot(ContainElement(stalledWorkerName))
				}
			})

			It("can be pruned while in stalled state", func() {
				fly("prune-worker", "-w", stalledWorkerName)
				waitForWorkersToBeRunning()
			})
		})
	})

	Context("with no other worker available", func() {
		BeforeEach(func() {
			Deploy("deployments/concourse.yml", "-o", "operations/forward-worker.yml")
		})

		Context("when the worker stalls while a build is running", func() {
			var buildSession *gexec.Session
			var buildID string

			BeforeEach(func() {
				buildSession = spawnFly("execute", "-c", "tasks/wait.yml")
				Eventually(buildSession).Should(gbytes.Say("executing build"))

				buildRegex := regexp.MustCompile(`executing build (\d+)`)
				matches := buildRegex.FindSubmatch(buildSession.Out.Contents())
				buildID = string(matches[1])

				Eventually(buildSession).Should(gbytes.Say("waiting for /tmp/stop-waiting"))

				By("stopping the worker without draining")
				bosh("ssh", "concourse/0", "-c", "sudo /var/vcap/bosh/bin/monit stop worker")
				bosh("ssh", "concourse/0", "-c", "sudo /var/vcap/bosh/bin/monit stop garden")

				By("waiting for it to stall")
				_ = waitForStalledWorker()
			})

			AfterEach(func() {
				buildSession.Signal(os.Interrupt)
				<-buildSession.Exited
			})

			Context("when the worker does not come back", func() {
				AfterEach(func() {
					bosh("ssh", "concourse/0", "-c", "sudo /var/vcap/bosh/bin/monit start worker")
					bosh("ssh", "concourse/0", "-c", "sudo /var/vcap/bosh/bin/monit start garden")
					waitForWorkersToBeRunning()
				})

				It("does not fail the build", func() {
					Consistently(buildSession).ShouldNot(gexec.Exit())
				})
			})

			Context("when the worker comes back", func() {
				It("resumes the build", func() {
					bosh("ssh", "concourse/0", "-c", "sudo /var/vcap/bosh/bin/monit start worker")
					bosh("ssh", "concourse/0", "-c", "sudo /var/vcap/bosh/bin/monit start garden")
					waitForWorkersToBeRunning()

					// Garden doesn't seem to stream output after restarting it. Guardian bug?
					// By("reattaching to the build")
					// _, err := ioutil.ReadAll(buildSession.Out)
					// Expect(err).ToNot(HaveOccurred())
					// Eventually(buildSession).Should(gbytes.Say("waiting for /tmp/stop-waiting"))

					By("hijacking the build to tell it to finish")
					Eventually(func() int {
						session := spawnFly(
							"hijack",
							"-b", buildID,
							"-s", "one-off",
							"touch", "/tmp/stop-waiting",
						)

						<-session.Exited
						return session.ExitCode()
					}).Should(Equal(0))

					By("waiting for the build to exit")
					// Garden doesn't seem to stream output after restarting it. Guardian bug?
					// Eventually(buildSession).Should(gbytes.Say("done"))
					<-buildSession.Exited
					Expect(buildSession.ExitCode()).To(Equal(0))
				})
			})
		})
	})
})
