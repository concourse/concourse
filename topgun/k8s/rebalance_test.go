package k8s_test

import (
	"fmt"
	"strings"
	"time"

	"github.com/onsi/gomega/gexec"

	. "github.com/concourse/concourse/topgun"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Worker Rebalancing", func() {
	var (
		releaseName  string
		proxySession *gexec.Session
		atcEndpoint  string
	)

	BeforeEach(func() {
		releaseName = fmt.Sprintf("topgun-worker-rebalancing-%d", GinkgoParallelNode())

		helmDeploy(releaseName,
			"--set=concourse.worker.ephemeral=true",
			"--set=worker.replicas=1",
			"--set=web.replicas=2",
			"--set=concourse.worker.rebalanceInterval=5s",
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

	Describe("when a rebalance time is configured", func() {
		It("the worker eventually connects to both web nodes over a period of time", func() {
			By("Logging in")
			fly.Login("test", "test", atcEndpoint)

			pods := getPods(releaseName, "--selector=app="+releaseName+"-web")

			Eventually(func() string {
				workers := fly.GetWorkers()
				Expect(workers).To(HaveLen(1))
				return strings.Split(workers[0].GardenAddress, ":")[0]
			}, 2*time.Minute, 10*time.Second).
				Should(Equal(pods[0].Status.Ip))

			Eventually(func() string {
				workers := fly.GetWorkers()

				Expect(workers).To(HaveLen(1))
				return strings.Split(workers[0].GardenAddress, ":")[0]
			}, 2*time.Minute, 10*time.Second).
				Should(Equal(pods[1].Status.Ip))
		})
	})
})

