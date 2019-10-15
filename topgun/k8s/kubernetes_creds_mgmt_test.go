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
	)

	BeforeEach(func() {
		setReleaseNameAndNamespace("k8s-cm")
	})

	JustBeforeEach(func() {
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

		BeforeEach(func() {
			deployConcourseChart(releaseName, "--set=worker.replicas=1")
		})

		It("Contains kubernetes config", func() {
			token, err := FetchToken(atcEndpoint, username, password)
			Expect(err).ToNot(HaveOccurred())

			body, err := RequestCredsInfo(atcEndpoint, token.AccessToken)
			Expect(err).ToNot(HaveOccurred())

			err = json.Unmarshal(body, &parsedResponse)
			Expect(err).ToNot(HaveOccurred())

			Expect(parsedResponse.Kubernetes.ConfigPath).To(BeEmpty())
			Expect(parsedResponse.Kubernetes.InClusterConfig).To(BeTrue())
			Expect(parsedResponse.Kubernetes.NamespaceConfig).To(Equal(releaseName + "-"))
		})
	})

	Context("Consuming k8s credentials", func() {
		var cachingSetup = func() {
			deployConcourseChart(releaseName, "--set=worker.replicas=1",
				"--set=concourse.web.secretCacheEnabled=true",
				"--set=concourse.web.secretCacheDuration=600",
			)
		}

		var disableTeamNamespaces = func() {
			By("creating a namespace made by the user instead of the chart")
			Run(nil, "kubectl", "create", "namespace", releaseName+"-main")

			deployConcourseChart(releaseName, "--set=worker.replicas=1",
				"--set=concourse.web.secretCacheEnabled=true",
				"--set=concourse.web.secretCacheDuration=600",
				"--set=concourse.web.kubernetes.createTeamNamespaces=false",
			)
		}

		Context("using per-team credentials", func() {
			secretNameFoo := "foo"
			secretNameCaz := "caz"

			Context("using the default namespace created by the chart", func() {
				BeforeEach(func() {
					deployConcourseChart(releaseName, "--set=worker.replicas=1")
				})

				It("succeeds", func() {
					runsBuildWithCredentialsResolved(secretNameFoo, secretNameCaz)
				})
			})

			Context("with caching enabled", func() {
				BeforeEach(cachingSetup)

				It("gets cached credentials", func() {
					runGetsCachedCredentials(secretNameFoo, secretNameCaz)
				})
			})

			Context("using a user-provided namespace", func() {
				BeforeEach(disableTeamNamespaces)

				It("succeeds", func() {
					runsBuildWithCredentialsResolved(secretNameFoo, secretNameCaz)
				})

				AfterEach(func() {
					Run(nil, "kubectl", "delete", "namespace", releaseName+"-main", "--wait=false")
				})
			})

		})

		Context("using per-pipeline credentials", func() {
			secretNameFoo := "pipeline.foo"
			secretNameCaz := "pipeline.caz"

			Context("using the default namespace created by the chart", func() {
				BeforeEach(func() {
					deployConcourseChart(releaseName, "--set=worker.replicas=1")
				})

				It("succeeds", func() {
					runsBuildWithCredentialsResolved(secretNameFoo, secretNameCaz)
				})
			})

			Context("with caching enabled", func() {
				BeforeEach(cachingSetup)

				It("gets cached credentials", func() {
					runGetsCachedCredentials(secretNameFoo, secretNameCaz)
				})
			})

			Context("using a user-provided namespace", func() {
				BeforeEach(disableTeamNamespaces)

				It("succeeds", func() {
					runsBuildWithCredentialsResolved(secretNameFoo, secretNameCaz)
				})

				AfterEach(func() {
					Run(nil, "kubectl", "delete", "namespace", releaseName+"-main", "--wait=false")
				})
			})
		})
	})

	Context("one-off build", func() {
		BeforeEach(func() {
			deployConcourseChart(releaseName, "--set=worker.replicas=1")
		})

		It("runs the one-off build successfully", func() {
			By("creating the secret in the main team")
			createCredentialSecret(releaseName, "some-secret", "main", map[string]string{"value": "mysecret"})

			By("successfully running the one-off build")
			fly.RunWithRetry("execute",
				"-c", "tasks/simple-secret.yml")
		})

		It("one-off build fails", func() {
			Eventually(func() int {
				sess := fly.Start("execute",
					"-c", "tasks/simple-secret.yml")
				<-sess.Exited
				return sess.ExitCode()
			}, 1*time.Minute).ShouldNot(BeZero())
			By("not creating the secret")
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

func runsBuildWithCredentialsResolved(normalSecret string, specialKeySecret string) {
	By("creating credentials in k8s credential manager")
	createCredentialSecret(releaseName, normalSecret, "main", map[string]string{"value": "bar"})
	createCredentialSecret(releaseName, specialKeySecret, "main", map[string]string{"baz": "zaz"})

	fly.RunWithRetry("set-pipeline", "-n",
		"-c", "pipelines/minimal-credential-management.yml",
		"-p", "pipeline",
	)

	fly.RunWithRetry("unpause-pipeline", "-p", "pipeline")

	var session *gexec.Session
	Eventually(func() int {
		session = fly.Start("trigger-job", "-j", "pipeline/unit", "-w")
		<-session.Exited
		return session.ExitCode()
	}, 1*time.Minute).Should(BeZero())

	By("seeing the credentials were resolved by concourse")
	Eventually(string(session.Out.Contents())).Should(ContainSubstring("bar"))
	Eventually(string(session.Out.Contents())).Should(ContainSubstring("zaz"))
}

func runGetsCachedCredentials(secretNameFoo string, secretNameCaz string) {
	runsBuildWithCredentialsResolved(secretNameFoo, secretNameCaz)
	deleteSecret(releaseName, "main", secretNameFoo)
	deleteSecret(releaseName, "main", secretNameCaz)
	By("seeing that concourse uses the cached credentials")
	runsBuildWithCredentialsResolved(secretNameFoo, secretNameCaz)
}
