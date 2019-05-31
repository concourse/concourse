package k8s_test

import (
	"fmt"
	"time"

	"github.com/onsi/gomega/gexec"

	. "github.com/concourse/concourse/v5/topgun"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Ephemeral workers", func() {
	var (
		proxySession *gexec.Session
		atcEndpoint  string
	)

	BeforeEach(func() {
		setReleaseNameAndNamespace("ew")

		deployConcourseChart(releaseName,
			// TODO: https://github.com/concourse/concourse/v5/issues/2827
			"--set=concourse.web.gc.interval=300ms",
			"--set=concourse.web.tsa.heartbeatInterval=300ms",
			"--set=worker.replicas=1",
			"--set=concourse.worker.baggageclaim.driver=overlay")

		waitAllPodsInNamespaceToBeReady(namespace)

		By("Creating the web proxy")
		proxySession, atcEndpoint = startPortForwarding(namespace, "service/"+releaseName+"-web", "8080")

		By("Logging in")
		fly.Login("test", "test", atcEndpoint)

		By("waiting for a running worker")
		Eventually(func() []Worker {
			return getRunningWorkers(fly.GetWorkers())
		}, 2*time.Minute, 10*time.Second).
			ShouldNot(HaveLen(0))
	})

	AfterEach(func() {
		cleanup(releaseName, namespace, proxySession)
	})

	It("Gets properly cleaned when getting removed and then put back on", func() {
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
