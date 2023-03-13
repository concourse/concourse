package k8s_test

import (
	"fmt"
	"time"

	. "github.com/concourse/concourse/topgun"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Ephemeral workers", func() {

	var atc Endpoint

	BeforeEach(func() {
		setReleaseNameAndNamespace("ew")

		deployConcourseChart(releaseName,
			// TODO: https://github.com/concourse/concourse/issues/2827
			"--set=concourse.web.gc.interval=300ms",
			"--set=concourse.web.tsa.heartbeatInterval=300ms",
			"--set=worker.replicas=1",
			"--set=concourse.worker.baggageclaim.driver=overlay")

		atc = waitAndLogin(namespace, releaseName+"-web")
	})

	AfterEach(func() {
		cleanupReleases()
		atc.Close()
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
