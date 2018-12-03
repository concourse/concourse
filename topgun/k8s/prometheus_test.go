package k8s_test

import (
	"encoding/json"
	"fmt"
	"github.com/onsi/gomega/gexec"
	"net/http"
	"time"

	. "github.com/concourse/concourse/topgun"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type prometheusMetrics struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"result_type"`
	} `json:"data"`
}

func getPrometheusMetrics(endpoint, releaseName string) (*prometheusMetrics, error) {
	req, err := http.NewRequest("GET", endpoint+"/api/v1/query", nil)
	if err != nil {
		return nil, err
	}

	req.URL.RawQuery = `query=concourse_db_connections%7Bapp%3D%22` +
		releaseName + `%22%7D`

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	metrics := new(prometheusMetrics)
	err = json.NewDecoder(resp.Body).Decode(metrics)
	if err != nil {
		return nil, err
	}

	return metrics, nil
}

var _ = Describe("Prometheus integration", func() {
	var (
		proxySession       *gexec.Session
		releaseName        string
		prometheusEndpoint string
	)

	BeforeEach(func() {
		releaseName = fmt.Sprintf("topgun-prometheus-integration-%d", GinkgoParallelNode())

		helmDeploy(releaseName,
			"--set=prometheus.enabled=true",
			"--set=worker.replicas=1",
			"--set=concourse.worker.ephemeral=true",
			"--set=concourse.web.prometheus.enabled=true",
			"--set=concourse.worker.baggageclaim.driver=detect")

		Eventually(func() bool {
			expectedPods := getPodsNames(getPods(releaseName))
			actualPods := getPodsNames(getPods(releaseName, "--field-selector=status.phase=Running"))

			return len(expectedPods) == len(actualPods)
		}, 5*time.Minute, 10*time.Second).Should(BeTrue(), "expected all pods to be running")

		By("Creating the prometheus proxy")
		proxySession, prometheusEndpoint = startPortForwarding(releaseName+"-server", "80")
	})

	AfterEach(func() {
		helmDestroy(releaseName)
		Wait(proxySession.Interrupt())
	})

	It("Is able to retrieve concourse metrics", func() {
		Eventually(func() bool {
			metrics, err := getPrometheusMetrics(prometheusEndpoint, releaseName)
			if err != nil {
				return false
			}

			if metrics.Status != "success" {
				return false
			}

			return true
		}, 2*time.Minute, 10*time.Second).Should(BeTrue(), "be able to retrieve metrics")
	})
})
