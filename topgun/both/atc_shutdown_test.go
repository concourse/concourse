package topgun_test

import (
	"regexp"
	"time"

	. "github.com/concourse/concourse/topgun/common"
	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("ATC Shutting down", func() {
	Context("with two atcs available", func() {
		var atcs []BoshInstance
		var atc0URL string
		var atc1URL string

		BeforeEach(func() {
			Deploy(
				"deployments/concourse.yml",
				"-o", "operations/web-instances.yml",
				"-v", "web_instances=2",
			)

			WaitForRunningWorker()

			atcs = JobInstances("web")
			atc0URL = "http://" + atcs[0].IP + ":8080"
			atc1URL = "http://" + atcs[1].IP + ":8080"

			Fly.Login(AtcUsername, AtcPassword, atc0URL)
		})

		Context("when one of the ATCS is stopped", func() {
			var stopSession *gexec.Session

			BeforeEach(func() {
				By("stopping one of the web instances")
				stopSession = SpawnBosh("stop", atcs[1].Name)
				Eventually(stopSession).Should(gexec.Exit(0))
			})

			AfterEach(func() {
				restartSession := SpawnBosh("start", atcs[0].Name)
				<-restartSession.Exited
				Eventually(restartSession).Should(gexec.Exit(0))
			})

			Describe("workers registering with random TSA address", func() {
				It("recovers from the TSA going down by registering with a random TSA", func() {
					WaitForRunningWorker()

					By("stopping the other web instance")
					stopSession = SpawnBosh("stop", atcs[0].Name)
					Eventually(stopSession).Should(gexec.Exit(0))

					By("starting the stopped web instance")
					startSession := SpawnBosh("start", atcs[1].Name)
					Eventually(startSession).Should(gexec.Exit(0))

					atcs = JobInstances("web")
					atc0URL = "http://" + atcs[0].IP + ":8080"
					atc1URL = "http://" + atcs[1].IP + ":8080"

					Fly.Login(AtcUsername, AtcPassword, atc1URL)

					WaitForRunningWorker()
				})
			})
		})

		Describe("tracking builds previously tracked by shutdown ATC", func() {
			var buildID string

			BeforeEach(func() {
				By("executing a task")
				buildSession := Fly.Start("execute", "-c", "tasks/wait.yml")
				Eventually(buildSession).Should(gbytes.Say("executing build"))

				buildRegex := regexp.MustCompile(`executing build (\d+)`)
				matches := buildRegex.FindSubmatch(buildSession.Out.Contents())
				buildID = string(matches[1])

				//For the initializing block
				Eventually(buildSession).Should(gbytes.Say("echo 'waiting for /tmp/stop-waiting to exist'"))
				//For the output from the running step
				Eventually(buildSession).Should(gbytes.Say("waiting for /tmp/stop-waiting to exist"))
			})

			Context("when the web node tracking the build shuts down", func() {
				JustBeforeEach(func() {
					By("restarting both web nodes")
					Bosh("restart", atcs[0].Group)
				})

				It("continues tracking the build progress", func() {
					By("waiting for another web node to attach to process")
					watchSession := Fly.Start("watch", "-b", buildID)
					Eventually(watchSession).Should(gbytes.Say("waiting for /tmp/stop-waiting"))
					time.Sleep(10 * time.Second)

					By("hijacking the build to tell it to finish")
					Eventually(func()int {
						hijackSession := Fly.Start(
							"hijack",
							"-b", buildID,
							"-s", "one-off", "--",
							"touch", "/tmp/stop-waiting",
						)
						<-hijackSession.Exited
						return hijackSession.ExitCode()
					}).Should(Equal(0))

					By("waiting for the build to exit")
					Eventually(watchSession, 1*time.Minute).Should(gbytes.Say("done"))
					<-watchSession.Exited
					Expect(watchSession.ExitCode()).To(Equal(0))
				})
			})
		})
	})
})
