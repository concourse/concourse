package k8s_test

import (
	"encoding/json"

	. "github.com/concourse/concourse/topgun"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Kubernetes credential management", func() {
	var (
		atc                Endpoint
		username, password = "test", "test"
	)

	BeforeEach(func() {
		setReleaseNameAndNamespace("k8s-cm")
	})

	AfterEach(func() {
		atc.Close()
		cleanupReleases()
	})

	JustBeforeEach(func() {
		atc = waitAndLogin(namespace, releaseName+"-web")
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
			token, err := FetchToken("http://"+atc.Address(), username, password)
			Expect(err).ToNot(HaveOccurred())

			body, err := RequestCredsInfo("http://"+atc.Address(), token.AccessToken)
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
				"--set=concourse.web.secretCacheDuration=600s",
			)
		}

		var disableTeamNamespaces = func() {
			By("creating a namespace made by the user instead of the chart")
			Run(nil, "kubectl", "create", "namespace", releaseName+"-main")

			By("binding the role that grants access to the secrets in our newly created namespace ")
			Run(nil,
				"kubectl", "create",
				"--namespace", releaseName+"-main",
				"rolebinding", "rb",
				"--clusterrole", releaseName+"-web",
				"--serviceaccount", releaseName+":"+releaseName+"-web",
			)

			deployConcourseChart(releaseName, "--set=worker.replicas=1",
				"--set=concourse.web.secretCacheEnabled=true",
				"--set=concourse.web.secretCacheDuration=600s",
				"--set=concourse.web.kubernetes.createTeamNamespaces=false",
			)
		}

		Context("using per-team credentials", func() {

			const (
				secretNameFoo = "foo"
				secretNameCaz = "caz"
			)

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

			const (
				secretNameFoo = "pipeline.foo"
				secretNameCaz = "pipeline.caz"
			)

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
			fly.Run("execute",
				"-c", "tasks/simple-secret.yml")
		})

		It("one-off build fails", func() {
			By("not creating the secret")
			sess := fly.Start("execute",
				"-c", "tasks/simple-secret.yml")
			<-sess.Exited
			Expect(sess.ExitCode()).NotTo(Equal(0))
		})
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

	fly.Run("set-pipeline", "-n",
		"-c", "pipelines/minimal-credential-management.yml",
		"-p", "pipeline",
	)

	fly.Run("unpause-pipeline", "-p", "pipeline")

	session := fly.Start("trigger-job", "-j", "pipeline/unit", "-w")
	Wait(session)

	By("seeing the credentials were resolved by concourse")
	Expect(string(session.Out.Contents())).To(ContainSubstring("bar"))
	Expect(string(session.Out.Contents())).To(ContainSubstring("zaz"))
}

func runGetsCachedCredentials(secretNameFoo string, secretNameCaz string) {
	runsBuildWithCredentialsResolved(secretNameFoo, secretNameCaz)
	deleteSecret(releaseName, "main", secretNameFoo)
	deleteSecret(releaseName, "main", secretNameCaz)
	By("seeing that concourse uses the cached credentials")
	runsBuildWithCredentialsResolved(secretNameFoo, secretNameCaz)
}
