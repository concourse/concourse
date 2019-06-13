package k8s_test

import (
	"encoding/json"
	"time"

	"github.com/onsi/gomega/gexec"

	. "github.com/concourse/concourse/topgun"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Kubernetes credential management", func() {
	var (
		proxySession *gexec.Session
		atcEndpoint  string
		username     = "test"
		password     = "test"
		extraArgs    []string
	)

	BeforeEach(func() {
		setReleaseNameAndNamespace("k8s-cm")
	})

	JustBeforeEach(func() {

		deployConcourseChart(releaseName, append([]string{
			"--set=worker.replicas=1",
		}, extraArgs...)...)

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



	Context("Consuming k8s credentials", func() {
		var runsBuildWithCredentialsResolved = func(normalSecret string, specialKeySecret string) {
			By("creating credentials in k8s credential manager")
			createCredentialSecret(releaseName, normalSecret, "main", map[string]string{"value": "bar"})
			createCredentialSecret(releaseName, specialKeySecret, "main", map[string]string{"baz": "zaz"})

			fly.Run("set-pipeline", "-n",
				"-c", "../pipelines/minimal-credential-management.yml",
				"-p", "pipeline",
			)

			fly.Run("unpause-pipeline", "-p", "pipeline")

			session := fly.Start("trigger-job", "-j", "pipeline/unit", "-w")
			Wait(session)

			By("seeing the credentials were resolved by concourse")
			Expect(string(session.Out.Contents())).To(ContainSubstring("bar"))
			Expect(string(session.Out.Contents())).To(ContainSubstring("zaz"))
		}

		var runGetsCachedCredentials = func(normalSecret string, specialKeySecret string) {
			runsBuildWithCredentialsResolved(normalSecret, specialKeySecret)
			deleteSecret(releaseName, "main", normalSecret)
			deleteSecret(releaseName, "main", specialKeySecret)
			By("seeing that concourse uses the cached credentials")
			runsBuildWithCredentialsResolved(normalSecret, specialKeySecret)
		}

		var cachingSetup = func() {
			extraArgs = []string{
				"--set=concourse.web.secretCacheEnabled=true",
				"--set=concourse.web.secretCacheDuration=600",
			}
		}

		var disableTeamNamspaces = func() {
			By("creating a namespace made by the user instead of the chart")
			Run(nil, "kubectl", "create", "namespace", releaseName+"-main")
			extraArgs = []string{
				"--set=concourse.web.kubernetes.createTeamNamespaces=false",
			}
		}

		var resetExtraArgs = func() {
			extraArgs = []string{}
		}


		Context("using per-team credentials", func() {
			normalSecret := "foo"
			specialKeySecret := "caz"

			Context("using the default namespace created by the chart", func() {
				BeforeEach(resetExtraArgs)

				It("succeeds", func() {
					runsBuildWithCredentialsResolved(normalSecret, specialKeySecret)
				})
			})

			Context("with caching enabled", func() {
				BeforeEach(cachingSetup)

				It("gets cached credentials", func() {
					runGetsCachedCredentials(normalSecret, specialKeySecret)
				})
			})

			Context("using a user-provided namespace", func() {
				BeforeEach(disableTeamNamspaces)

				It("succeeds", func() {
					runsBuildWithCredentialsResolved(normalSecret, specialKeySecret)
				})

				AfterEach(func() {
					Run(nil, "kubectl", "delete", "namespace", releaseName+"-main", "--wait=false")
				})
			})

		})

		Context("using per-pipeline credentials", func() {
			normalSecret := "pipeline.foo"
			specialKeySecret := "pipeline.caz"

			Context("using the default namespace created by the chart", func() {
				BeforeEach(resetExtraArgs)

				It("succeeds", func() {
					runsBuildWithCredentialsResolved(normalSecret, specialKeySecret)
				})
			})

			Context("with caching enabled", func() {
				BeforeEach(cachingSetup)

				It("gets cached credentials", func() {
					runGetsCachedCredentials(normalSecret, specialKeySecret)
				})
			})

			Context("using a user-provided namespace", func() {
				BeforeEach(disableTeamNamspaces)

				It("succeeds", func() {
					runsBuildWithCredentialsResolved(normalSecret, specialKeySecret)
				})

				AfterEach(func() {
					Run(nil, "kubectl", "delete", "namespace", releaseName+"-main", "--wait=false")
				})
			})
		})
	})

	AfterEach(func() {
		cleanup(releaseName, namespace, proxySession)
	})

})

func deleteSecret(releaseName, team, secretName string) {
	Run(nil, "kubectl", "--namespace="+releaseName+"-main", "delete", "secret", secretName)
}

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
