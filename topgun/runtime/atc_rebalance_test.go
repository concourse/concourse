package topgun_test

import (
	"os"
	"strings"
	"time"

	. "github.com/concourse/concourse/topgun/common"
	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Rebalancing workers", func() {
	var rebalanceInterval = 5 * time.Second

	Context("with two TSAs available", func() {
		var webInstances []BoshInstance

		BeforeEach(func() {
			Deploy(
				"deployments/concourse.yml",
				"-o", "operations/web-instances.yml",
				"-v", "web_instances=2",
				"-o", "operations/worker-rebalancing.yml",
				"-v", "rebalance_interval="+rebalanceInterval.String(),
			)

			WaitForRunningWorker()

			webInstances = JobInstances("web")
		})

		It("rotates the worker to between both web nodes over a period of time", func() {
			Eventually(func() string {
				workers := FlyTable("workers", "-d")
				return strings.Split(workers[0]["garden address"], ":")[0]
			}).Should(SatisfyAny(
				Equal(webInstances[0].IP),
				Equal(webInstances[0].DNS),
			))

			Eventually(func() string {
				workers := FlyTable("workers", "-d")
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
				buildSession = Fly.Start("execute", "-c", "tasks/wait.yml")
				Eventually(buildSession).Should(gbytes.Say("executing build"))
				Eventually(buildSession).Should(gbytes.Say("waiting for /tmp/stop-waiting"))

				boshStopSession = SpawnBosh("stop", "worker/0")
				Eventually(WaitForWorkerInState("retiring")).ShouldNot(BeEmpty())
			})

			AfterEach(func() {
				buildSession.Signal(os.Interrupt)
				<-buildSession.Exited

				<-boshStopSession.Exited
				Bosh("start", "worker/0")
			})

			It("does not rebalance", func() {
				originalAddr := FlyTable("workers", "-d")[0]["garden address"]
				Expect(originalAddr).ToNot(BeEmpty())

				Consistently(func() string {
					worker := FlyTable("workers", "-d")[0]
					Expect(worker["state"]).To(Equal("retiring"))

					return worker["garden address"]
				}, 3*rebalanceInterval).Should(Equal(originalAddr))
			})
		})
	})
})
