package k8s_test

import (
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Worker Rebalancing", func() {

	var atc Endpoint

	BeforeEach(func() {
		setReleaseNameAndNamespace("wr")

		deployConcourseChart(releaseName,
			"--set=concourse.worker.ephemeral=true",
			"--set=worker.replicas=1",
			"--set=web.replicas=2",
			"--set=concourse.worker.rebalanceInterval=5s",
			"--set=concourse.worker.baggageclaim.driver=detect")

		atc = waitAndLogin(namespace, releaseName+"-web")
	})

	AfterEach(func() {
		atc.Close()
		cleanup(releaseName, namespace)
	})

	It("eventually has worker connecting to each web nodes over a period of time", func() {
		pods := getPods(releaseName, metav1.ListOptions{LabelSelector: "app=" + releaseName + "-web"})

		Eventually(func() string {
			workers := fly.GetWorkers()
			Expect(workers).To(HaveLen(1))

			return strings.Split(workers[0].GardenAddress, ":")[0]
		}, 2*time.Minute, 10*time.Second).
			Should(Equal(pods[0].Status.PodIP))

		Eventually(func() string {
			workers := fly.GetWorkers()

			Expect(workers).To(HaveLen(1))
			return strings.Split(workers[0].GardenAddress, ":")[0]
		}, 2*time.Minute, 10*time.Second).
			Should(Equal(pods[1].Status.PodIP))
	})
})
