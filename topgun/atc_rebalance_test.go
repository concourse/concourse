package topgun_test

import (
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Rebalancing workers", func() {
	var rebalanceInterval = 5 * time.Second

	Context("with two TSAs available", func() {
		var webInstances []boshInstance

		BeforeEach(func() {
			Deploy(
				"deployments/concourse.yml",
				"-o", "operations/web-instances.yml",
				"-v", "web_instances=2",
				"-o", "operations/worker-rebalancing.yml",
				"-v", "rebalance_interval="+rebalanceInterval.String(),
			)

			waitForRunningWorker()

			webInstances = JobInstances("web")
		})

		It("rotates the worker to between both web nodes over a period of time", func() {
			Eventually(func() string {
				workers := flyTable("workers", "-d")
				return strings.Split(workers[0]["garden address"], ":")[0]
			}).Should(SatisfyAny(
				Equal(webInstances[0].IP),
				Equal(webInstances[0].DNS),
			))

			Eventually(func() string {
				workers := flyTable("workers", "-d")
				return strings.Split(workers[0]["garden address"], ":")[0]
			}).Should(SatisfyAny(
				Equal(webInstances[1].IP),
				Equal(webInstances[1].DNS),
			))
		})

		Context("while the worker is draining", func() {
			var buildSession *gexec.Session
			var boshStopSession *gexec.Session

			BeforeEach(func() {
				buildSession = fly.Start("execute", "-c", "tasks/wait.yml")
				Eventually(buildSession).Should(gbytes.Say("executing build"))
				Eventually(buildSession).Should(gbytes.Say("waiting for /tmp/stop-waiting"))

				boshStopSession = spawnBosh("stop", "worker/0")
				Eventually(waitForWorkerInState("retiring")).ShouldNot(BeEmpty())
			})

			AfterEach(func() {
				buildSession.Signal(os.Interrupt)
				<-buildSession.Exited

				<-boshStopSession.Exited
				bosh("start", "worker/0")
			})

			It("does not rebalance", func() {
				originalAddr := flyTable("workers", "-d")[0]["garden address"]
				Expect(originalAddr).ToNot(BeEmpty())

				Consistently(func() string {
					worker := flyTable("workers", "-d")[0]
					Expect(worker["state"]).To(Equal("retiring"))

					return worker["garden address"]
				}, 3*rebalanceInterval).Should(Equal(originalAddr))
			})
		})
	})
})
