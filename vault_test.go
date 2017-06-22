package topgun_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Vault", func() {
	pgDump := func() *gexec.Session {
		dump := exec.Command("pg_dump", "-U", "atc", "-h", dbInstance.IP, "atc")
		dump.Env = append(os.Environ(), "PGPASSWORD=dummy-password")
		dump.Stdin = bytes.NewBufferString("dummy-password\n")
		session, err := gexec.Start(dump, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
		<-session.Exited
		Expect(session.ExitCode()).To(Equal(0))
		return session
	}

	getPipeline := func() *gexec.Session {
		session := spawnFly("get-pipeline", "-p", "pipeline-vault-test")
		<-session.Exited
		Expect(session.ExitCode()).To(Equal(0))
		return session
	}

	Describe("A deployment with vault", func() {
		var vaultURL string
		var vaultToken string
		var vaultCACert string

		vault := func(command string, args ...string) *gexec.Session {
			cmd := exec.Command("vault", append([]string{command}, args...)...)
			cmd.Env = append(
				os.Environ(),
				"VAULT_ADDR="+vaultURL,
				"VAULT_TOKEN="+vaultToken,
			)

			if vaultCACert != "" {
				cmd.Env = append(
					cmd.Env,
					"VAULT_CACERT="+vaultCACert,
					"VAULT_SKIP_VERIFY=true",
				)
			}

			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			wait(session)
			return session
		}

		BeforeEach(func() {
			vaultURL = ""
			vaultToken = ""
			vaultCACert = ""

			varsStore, err := ioutil.TempFile("", "vars-store.yml")
			Expect(err).ToNot(HaveOccurred())
			Expect(varsStore.Close()).To(Succeed())

			defer os.Remove(varsStore.Name())

			Deploy(
				"deployments/vault.yml",
				"--vars-store", varsStore.Name(),
			)

			vaultInstance := JobInstance("vault")
			vaultURL = "http://" + vaultInstance.IP + ":8200"

			init := vault("init")
			content := string(init.Out.Contents())
			key1 := regexp.MustCompile(`Unseal Key 1: (.*)`).FindStringSubmatch(content)[1]
			key2 := regexp.MustCompile(`Unseal Key 2: (.*)`).FindStringSubmatch(content)[1]
			key3 := regexp.MustCompile(`Unseal Key 3: (.*)`).FindStringSubmatch(content)[1]

			vaultToken = regexp.MustCompile(`Initial Root Token: (.*)`).FindStringSubmatch(content)[1]

			vault("unseal", key1)
			vault("unseal", key2)
			vault("unseal", key3)

			vault("mount", "-path", "concourse/main", "generic")

			Deploy(
				"deployments/vault-with-concourse.yml",
				"--vars-store", varsStore.Name(),
				"-v", "vault-url="+vaultURL,
				"-v", "vault_ip="+vaultInstance.IP,
				"-v", "instances=0",
			)

			vaultURL = "https://" + vaultInstance.IP + ":8200"

			vaultCACertFile, err := ioutil.TempFile("", "vault-ca.cert")
			Expect(err).ToNot(HaveOccurred())

			vaultCACert = vaultCACertFile.Name()

			session := bosh("interpolate", "--path", "/vault_ca/certificate", varsStore.Name())
			_, err = fmt.Fprintf(vaultCACertFile, "%s", session.Out.Contents())
			Expect(err).ToNot(HaveOccurred())
			Expect(vaultCACertFile.Close()).To(Succeed())

			vault("unseal", key1)
			vault("unseal", key2)
			vault("unseal", key3)

			policyFile, err := ioutil.TempFile("", "vault-policy.hcl")
			Expect(err).ToNot(HaveOccurred())

			defer os.Remove(policyFile.Name())

			_, err = fmt.Fprintf(policyFile, "%s", `path "concourse/*" { policy = "read" }`)
			Expect(err).ToNot(HaveOccurred())

			Expect(policyFile.Close()).To(Succeed())

			vault("policy-write", "concourse", policyFile.Name())

			vault("auth-enable", "cert")

			vault(
				"write",
				"auth/cert/certs/concourse",
				"display_name=concourse",
				"certificate=@"+vaultCACert,
				"policies=concourse",
			)

			Deploy(
				"deployments/vault-with-concourse.yml",
				"--vars-store", varsStore.Name(),
				"-v", "vault-url="+vaultURL,
				"-v", "vault_ip="+vaultInstance.IP,
				"-v", "instances=1",
			)
		})

		AfterEach(func() {
			Expect(os.Remove(vaultCACert)).To(Succeed())
		})

		Context("with a pipeline build", func() {
			BeforeEach(func() {
				vault("write", "concourse/main/pipeline-vault-test/resource-type-repository", "value=concourse/time-resource")
				vault("write", "concourse/main/pipeline-vault-test/time-resource-interval", "value=10m")
				vault("write", "concourse/main/pipeline-vault-test/job-secret", "value=Hello")
				vault("write", "concourse/main/team-secret", "value=Sauce")
				vault("write", "concourse/main/pipeline-vault-test/image-resource-repository", "value=busybox")
			})

			It("parameterizes via Vault and leaves the pipeline uninterpolated", func() {
				By("setting a pipeline that contains vault secrets")
				fly("set-pipeline", "-n", "-c", "pipelines/vault.yml", "-p", "pipeline-vault-test")

				By("getting the pipeline config")
				session := getPipeline()
				Expect(string(session.Out.Contents())).ToNot(ContainSubstring("concourse/time-resource"))
				Expect(string(session.Out.Contents())).ToNot(ContainSubstring("10m"))
				Expect(string(session.Out.Contents())).ToNot(ContainSubstring("Hello"))
				Expect(string(session.Out.Contents())).ToNot(ContainSubstring("Sauce"))
				Expect(string(session.Out.Contents())).ToNot(ContainSubstring("busybox"))

				By("unpausing the pipeline")
				fly("unpause-pipeline", "-p", "pipeline-vault-test")

				By("triggering job")
				watch := spawnFly("trigger-job", "-w", "-j", "pipeline-vault-test/simple-job")
				wait(watch)
				Expect(watch).To(gbytes.Say("SECRET: Hello"))
				Expect(watch).To(gbytes.Say("TEAM SECRET: Sauce"))

				By("taking a dump")
				session = pgDump()
				Expect(session).ToNot(gbytes.Say("concourse/time-resource"))
				Expect(session).ToNot(gbytes.Say("10m"))
				Expect(session).To(gbytes.Say("Hello")) // build echoed it; nothing we can do
				Expect(session).To(gbytes.Say("Sauce")) // build echoed it; nothing we can do
				Expect(session).ToNot(gbytes.Say("busybox"))
			})
		})

		Context("with a one-off build", func() {
			BeforeEach(func() {
				vault("write", "concourse/main/task-secret", "value=Hello")
				vault("write", "concourse/main/image-resource-repository", "value=busybox")
			})

			It("parameterizes image_resource and params in a task config", func() {
				By("executing a task that parameterizes image_resource")
				watch := spawnFly("execute", "-c", "tasks/vault.yml")
				wait(watch)
				Expect(watch).To(gbytes.Say("SECRET: Hello"))

				By("taking a dump")
				session := pgDump()
				Expect(session).ToNot(gbytes.Say("concourse/time-resource"))
				Expect(session).To(gbytes.Say("Hello")) // build echoed it; nothing we can do
			})
		})
	})
})
