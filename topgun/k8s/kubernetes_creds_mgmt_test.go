package k8s_test

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/onsi/gomega/gexec"

	. "github.com/concourse/concourse/topgun"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Kubernetes credential management", func() {
	var (
		proxySession *gexec.Session
		releaseName  string
		atcEndpoint  string
		namespace    string
		username     = "test"
		password     = "test"
	)

	BeforeEach(func() {
		releaseName = fmt.Sprintf("topgun-k8s-cm-%d-%d", GinkgoRandomSeed(), GinkgoParallelNode())
		namespace = releaseName

		deployConcourseChart(releaseName, "--set=worker.replicas=1")

		waitAllPodsInNamespaceToBeReady(namespace)

		By("Creating the web proxy")
		proxySession, atcEndpoint = startPortForwarding(namespace, releaseName+"-web", "8080")

		By("Logging in")
		fly.Login(username, password, atcEndpoint)

		By("Waiting for a running worker")
		Eventually(func() []Worker {
			return getRunningWorkers(fly.GetWorkers())
		}, 2*time.Minute, 10*time.Second).
			ShouldNot(HaveLen(0))
	})

	Context("/api/v1/info/creds", func() {
		var parsedResponse struct {
			Kubernetes struct {
				ConfigPath      string `json:"config_path"`
				InClusterConfig bool   `json:"in_cluster_config"`
				NamespaceConfig string `json:"namespace_config"`
			} `json:"kubernetes"`
		}

		JustBeforeEach(func() {
			token, err := FetchToken(atcEndpoint, username, password)
			Expect(err).ToNot(HaveOccurred())

			body, err := RequestCredsInfo(atcEndpoint, token.AccessToken)
			Expect(err).ToNot(HaveOccurred())

			err = json.Unmarshal(body, &parsedResponse)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Contains kubernetes config", func() {
			Expect(parsedResponse.Kubernetes.ConfigPath).To(BeEmpty())
			Expect(parsedResponse.Kubernetes.InClusterConfig).To(BeTrue())
			Expect(parsedResponse.Kubernetes.NamespaceConfig).To(Equal(releaseName + "-"))
		})
	})

	Context("Consuming per-team k8s secrets", func() {
		BeforeEach(func() {
			// ((foo)) --> bar
			createCredentialSecret(releaseName, "foo", "main", map[string]string{"value": "bar"})

			// ((caz.baz)) --> zaz
			createCredentialSecret(releaseName, "caz", "main", map[string]string{"baz": "zaz"})

			fly.Run("set-pipeline", "-n", "-c", "../pipelines/minimal-credential-management.yml", "-p", "pipeline")
			session := fly.Start("get-pipeline", "-p", "pipeline")
			Wait(session)

			Expect(string(session.Out.Contents())).ToNot(ContainSubstring("bar"))
			Expect(string(session.Out.Contents())).ToNot(ContainSubstring("zaz"))

			fly.Run("unpause-pipeline", "-p", "pipeline")
		})

		It("Gets credentials set by consuming k8s secrets", func() {
			session := fly.Start("trigger-job", "-j", "pipeline/unit", "-w")
			Wait(session)

			Expect(string(session.Out.Contents())).To(ContainSubstring("bar"))
			Expect(string(session.Out.Contents())).To(ContainSubstring("zaz"))
		})
	})

	AfterEach(func() {
		helmDestroy(releaseName)
		Wait(proxySession.Interrupt())
		Wait(Start(nil, "kubectl", "delete", "namespace", namespace, "--wait=false"))
	})

})

func createCredentialSecret(releaseName, secretName, team string, kv map[string]string) {
	args := []string{
		"create",
		"secret",
		"generic",
		secretName,
		"--namespace=" + releaseName + "-" + team,
	}

	for key, value := range kv {
		args = append(args, "--from-literal="+key+"="+value)
	}

	Wait(Start(nil, "kubectl", args...))
}

