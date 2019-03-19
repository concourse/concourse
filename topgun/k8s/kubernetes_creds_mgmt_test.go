package k8s_test

import (
	"encoding/json"
	"fmt"
	"math/rand"
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
		extraArgs    []string
	)

	BeforeEach(func() {
		releaseName = fmt.Sprintf("topgun-k8s-cm-%d-%d", rand.Int(), GinkgoParallelNode())
		namespace = releaseName
	})

	JustBeforeEach(func() {

		deployConcourseChart(releaseName, append([]string{"--set=worker.replicas=1"}, extraArgs...)...)

		waitAllPodsInNamespaceToBeReady(namespace)

		By("Creating the web proxy")
		proxySession, atcEndpoint = startPortForwarding(namespace, "service/"+releaseName+"-web", "8080")

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
		JustBeforeEach(func() {
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

		Context("using the default namespace created by the chart", func() {
			It("Gets credentials set by consuming k8s secrets", func() {
				session := fly.Start("trigger-job", "-j", "pipeline/unit", "-w")
				Wait(session)

				Expect(string(session.Out.Contents())).To(ContainSubstring("bar"))
				Expect(string(session.Out.Contents())).To(ContainSubstring("zaz"))
			})
		})

		Context("using a user-provided namespace", func() {
			BeforeEach(func() {
				Run(nil, "kubectl", "create", "namespace", releaseName+"-main")
				extraArgs = []string{
					"--set=concourse.web.kubernetes.createTeamNamespaces=false",
				}
			})

			It("Gets credentials set by consuming k8s secrets", func() {
				session := fly.Start("trigger-job", "-j", "pipeline/unit", "-w")
				Wait(session)

				Expect(string(session.Out.Contents())).To(ContainSubstring("bar"))
				Expect(string(session.Out.Contents())).To(ContainSubstring("zaz"))
			})

			AfterEach(func() {
				Run(nil, "kubectl", "delete", "namespace", releaseName+"-main", "--wait=false")
			})
		})
	})

	AfterEach(func() {
		helmDestroy(releaseName)
		Wait(proxySession.Interrupt())
		Run(nil, "kubectl", "delete", "namespace", namespace)
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

	Run(nil, "kubectl", args...)
}
