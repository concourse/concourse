package k8s_test

import (
	"encoding/json"
	"net/http"

	. "github.com/concourse/concourse/topgun"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Garden Config", func() {

	var (
		garden              Endpoint
		helmDeployTestFlags []string
	)

	BeforeEach(func() {
		setReleaseNameAndNamespace("gc")
	})

	JustBeforeEach(func() {
		deployConcourseChart(releaseName, helmDeployTestFlags...)

		waitAllPodsInNamespaceToBeReady(namespace)

		pods := getPods(namespace, metav1.ListOptions{LabelSelector: "app=" + releaseName + "-worker"})
		Expect(pods).To(HaveLen(1))

		garden = endpointFactory.NewPodEndpoint(
			namespace,
			pods[0].ObjectMeta.Name,
			"7777",
		)
	})

	AfterEach(func() {
		garden.Close()
		cleanup(releaseName, namespace)
	})

	Context("passing a config map location to the worker to be used by gdn", func() {
		BeforeEach(func() {
			helmDeployTestFlags = []string{
				`--set=worker.replicas=1`,
				`--set=worker.additionalVolumes[0].name=garden-config`,
				`--set=worker.additionalVolumes[0].configMap.name=garden-config`,
				`--set=worker.additionalVolumeMounts[0].name=garden-config`,
				`--set=worker.additionalVolumeMounts[0].mountPath=/foo`,
				`--set=concourse.worker.garden.config=/foo/garden-config.ini`,
			}

			configMapCreationArgs := []string{
				"create",
				"configmap",
				"garden-config",
				"--namespace=" + namespace,
				`--from-literal=garden-config.ini=
[server]
  max-containers = 100`,
			}

			Run(nil, "kubectl", "create", "namespace", namespace)
			Run(nil, "kubectl", configMapCreationArgs...)
		})

		It("returns the configured number of max containers", func() {
			Expect(getMaxContainers(garden.Address())).To(Equal(100))
		})
	})

	Context("passing the CONCOURSE_GARDEN_ env vars to the gdn server", func() {
		BeforeEach(func() {
			helmDeployTestFlags = []string{
				`--set=worker.replicas=1`,
				`--set=worker.env[0].name=CONCOURSE_GARDEN_MAX_CONTAINERS`,
				`--set=worker.env[0].value="100"`,
			}
		})

		It("returns the configured number of max containers", func() {
			Expect(getMaxContainers(garden.Address())).To(Equal(100))
		})
	})

	Context("passing the CONCOURSE_GARDEN_DENY_NETWORK env var to the gdn server", func() {
		BeforeEach(func() {
			helmDeployTestFlags = []string{
				`--set=worker.replicas=1`,
				`--set=worker.env[0].name=CONCOURSE_GARDEN_DENY_NETWORK`,
				`--set=worker.env[0].value="8.8.8.8/24"`,
			}
		})

		It("causes requests to the specified IP range to fail", func() {
			atc := waitAndLogin(namespace, releaseName+"-web")
			defer atc.Close()
			buildSession := fly.Start("execute", "-c", "tasks/garden-deny-network.yml")
			<-buildSession.Exited

			Expect(buildSession.ExitCode()).NotTo(Equal(0))
		})
	})
})

type gardenCap struct {
	MaxContainers int `json:"max_containers"`
}

func getMaxContainers(addr string) int {
	req, err := http.NewRequest("GET", "http://"+addr+"/capacity", nil)
	Expect(err).ToNot(HaveOccurred())

	resp, err := http.DefaultClient.Do(req)
	Expect(err).ToNot(HaveOccurred())

	defer resp.Body.Close()

	gardenCapObject := gardenCap{}

	err = json.NewDecoder(resp.Body).Decode(&gardenCapObject)
	Expect(err).ToNot(HaveOccurred())

	return gardenCapObject.MaxContainers
}
