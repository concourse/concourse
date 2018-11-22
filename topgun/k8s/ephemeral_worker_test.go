package k8s_test

import (
	"fmt"
	"time"

	"github.com/onsi/gomega/gexec"

	. "github.com/concourse/concourse/topgun"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Ephemeral workers", func() {
	var (
		proxySession *gexec.Session
		releaseName  string
		atcEndpoint  string
	)

	BeforeEach(func() {
		releaseName = fmt.Sprintf("topgun-ephemeral-workers-%d", GinkgoParallelNode())

		helmDeploy(releaseName,
			// TODO: https://github.com/concourse/concourse/issues/2827
			"--set=concourse.web.gc.interval=300ms",
			"--set=concourse.web.tsa.heartbeatInterval=300ms",
			"--set=concourse.worker.ephemeral=true",
			"--set=worker.replicas=1",
			"--set=concourse.worker.baggageclaim.driver=detect")

		Eventually(func() bool {
			expectedPods := getPodsNames(releaseName)
			actualPods := getRunningPods(releaseName)

			return len(expectedPods) == len(actualPods)
		}, 5*time.Minute, 10*time.Second).Should(BeTrue(), "expected all pods to be running")

		By("Creating the web proxy")
		proxySession, atcEndpoint = startAtcServiceProxy(releaseName)
	})

	AfterEach(func() {
		helmDestroy(releaseName)
		Wait(proxySession.Interrupt())
	})

	It("Gets properly cleaned when getting removed and then put back on", func() {
		By("Logging in")
		fly.Login("test", "test", atcEndpoint)

		By("waiting for a running worker")
		Eventually(func() []Worker {
			return getRunningWorkers(fly.GetWorkers())
		}, 2*time.Minute, 10*time.Second).
			ShouldNot(HaveLen(0))

		deletePods(releaseName, fmt.Sprintf("--selector=app=%s-worker", releaseName))

		Eventually(func() (runningWorkers []Worker) {
			workers := fly.GetWorkers()
			for _, w := range workers {
				Expect(w.State).ToNot(Equal("stalled"), "the worker should never stall")
				if w.State == "running" {
					runningWorkers = append(runningWorkers, w)
				}
			}
			return
		}, 1*time.Minute, 1*time.Second).Should(HaveLen(0), "the running worker should go away")
	})
})

