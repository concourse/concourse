package k8s_test

import (
	"encoding/json"
	"net/http"
	"path"
	"time"

	. "github.com/concourse/concourse/topgun"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Prometheus integration", func() {

	var prometheusReleaseName string

	BeforeEach(func() {
		setReleaseNameAndNamespace("pi")
		prometheusReleaseName = releaseName + "-prom"

		deployConcourseChart(releaseName,
			"--set=worker.enabled=false",
			"--set=concourse.web.prometheus.enabled=true")

		Run(nil,
			"helm", "dependency", "update",
			path.Join(Environment.HelmChartsDir, "stable/prometheus"),
		)

		helmDeploy(prometheusReleaseName,
			namespace,
			path.Join(Environment.HelmChartsDir, "stable/prometheus"),
			"--set=nodeExporter.enabled=false",
			"--set=kubeStateMetrics.enabled=false",
			"--set=pushgateway.enabled=false",
			"--set=alertmanager.enabled=false",
			"--set=server.persistentVolume.enabled=false")

		waitAllPodsInNamespaceToBeReady(namespace)
	})

	AfterEach(func() {
		cleanupReleases()
	})

	It("Is able to retrieve concourse metrics", func() {
		prometheus := endpointFactory.NewServiceEndpoint(
			namespace,
			prometheusReleaseName+"-prometheus-server",
			"80",
		)
		defer prometheus.Close()

		Eventually(func() bool {
			metrics, err := getPrometheusMetrics("http://"+prometheus.Address(), releaseName)
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
