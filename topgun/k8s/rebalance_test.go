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
		namespace    string
		proxySession *gexec.Session
		atcEndpoint  string
	)

	BeforeEach(func() {
		releaseName = fmt.Sprintf("topgun-wr-%d-%d", GinkgoRandomSeed(), GinkgoParallelNode())
		namespace = releaseName

		deployConcourseChart(releaseName,
			"--set=concourse.worker.ephemeral=true",
			"--set=worker.replicas=1",
			"--set=web.replicas=2",
			"--set=concourse.worker.rebalanceInterval=5s",
			"--set=concourse.worker.baggageclaim.driver=detect")

		waitAllPodsInNamespaceToBeReady(namespace)

		By("Creating the web proxy")
		proxySession, atcEndpoint = startPortForwarding(namespace, releaseName+"-web", "8080")

		By("Logging in")
		fly.Login("test", "test", atcEndpoint)

		By("waiting for a running worker")
		Eventually(func() []Worker {
			return getRunningWorkers(fly.GetWorkers())
		}, 2*time.Minute, 10*time.Second).
			ShouldNot(HaveLen(0))
	})

	AfterEach(func() {
		helmDestroy(releaseName)
		Wait(Start(nil, "kubectl", "delete", "namespace", namespace, "--wait=false"))
		Wait(proxySession.Interrupt())
	})

	It("eventually has worker connecting to each web nodes over a period of time", func() {
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
