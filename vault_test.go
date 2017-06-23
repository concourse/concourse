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
		var (
			v         *vault
			varsStore *os.File
		)

		BeforeEach(func() {
			var err error

			varsStore, err = ioutil.TempFile("", "vars-store.yml")
			Expect(err).ToNot(HaveOccurred())
			Expect(varsStore.Close()).To(Succeed())

			Deploy(
				"deployments/vault-with-concourse.yml",
				"-v", "instances=0",
			)

			vaultInstance := JobInstance("vault")
			v = newVault(vaultInstance.IP)
		})

		AfterEach(func() {
			Expect(os.Remove(varsStore.Name())).To(Succeed())
		})

		testVaultIntegration := func() {
			Context("with a pipeline build", func() {
				BeforeEach(func() {
					v.Run("write", "concourse/main/pipeline-vault-test/resource_type_repository", "value=concourse/time-resource")
					v.Run("write", "concourse/main/pipeline-vault-test/time_resource_interval", "value=10m")
					v.Run("write", "concourse/main/pipeline-vault-test/job_secret", "value=Hello")
					v.Run("write", "concourse/main/team_secret", "value=Sauce")
					v.Run("write", "concourse/main/pipeline-vault-test/image_resource_repository", "value=busybox")
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
					v.Run("write", "concourse/main/task_secret", "value=Hello")
					v.Run("write", "concourse/main/image_resource_repository", "value=busybox")
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
		}

		Context("with TLS auth", func() {
			BeforeEach(func() {
				Deploy(
					"deployments/vault-with-concourse.yml",
					"--vars-store", varsStore.Name(),
					"-o", "deployments/ops/enable-tls.yml",
					"-v", "vault_url="+v.URI(),
					"-v", "vault_ip="+v.IP(),
					"-v", "instances=0",
				)

				vaultCACertFile, err := ioutil.TempFile("", "vault-ca.cert")
				Expect(err).ToNot(HaveOccurred())

				vaultCACert := vaultCACertFile.Name()

				session := bosh("interpolate", "--path", "/vault_ca/certificate", varsStore.Name())
				_, err = fmt.Fprintf(vaultCACertFile, "%s", session.Out.Contents())
				Expect(err).ToNot(HaveOccurred())
				Expect(vaultCACertFile.Close()).To(Succeed())

				v.SetCA(vaultCACert)
				v.Unseal()

				policyFile, err := ioutil.TempFile("", "vault-policy.hcl")
				Expect(err).ToNot(HaveOccurred())

				defer os.Remove(policyFile.Name())

				_, err = fmt.Fprintf(policyFile, "%s", `path "concourse/*" { policy = "read" }`)
				Expect(err).ToNot(HaveOccurred())

				Expect(policyFile.Close()).To(Succeed())

				v.Run("policy-write", "concourse", policyFile.Name())
				v.Run("auth-enable", "cert")
				v.Run(
					"write",
					"auth/cert/certs/concourse",
					"display_name=concourse",
					"certificate=@"+vaultCACert,
					"policies=concourse",
				)

				Deploy(
					"deployments/vault-with-concourse.yml",
					"--vars-store", varsStore.Name(),
					"-o", "deployments/ops/enable-tls.yml",
					"-v", "vault_url="+v.URI(),
					"-v", "vault_ip="+v.IP(),
					"-v", "instances=1",
				)
			})

			testVaultIntegration()
		})

		Context("with token auth", func() {
			BeforeEach(func() {
				Deploy(
					"deployments/vault-with-concourse.yml",
					"--vars-store", varsStore.Name(),
					"-v", "vault_url="+v.URI(),
					"-v", "vault_ip="+v.IP(),
					"-v", "instances=1",
					"-v", "vault_client_token="+v.ClientToken(),
				)
			})

			testVaultIntegration()
		})
	})
})

type vault struct {
	ip               string
	key1, key2, key3 string
	token            string
	caCert           string
}

func newVault(ip string) *vault {
	v := &vault{
		ip: ip,
	}
	v.init()
	return v
}

func (v *vault) SetCA(filename string) { v.caCert = filename }
func (v *vault) IP() string            { return v.ip }
func (v *vault) ClientToken() string   { return v.token }
func (v *vault) URI() string {
	if v.caCert == "" {
		return "http://" + v.ip + ":8200"
	}

	return "https://" + v.ip + ":8200"
}

func (v *vault) Run(command string, args ...string) *gexec.Session {
	cmd := exec.Command("vault", append([]string{command}, args...)...)
	cmd.Env = append(
		os.Environ(),
		"VAULT_ADDR="+v.URI(),
		"VAULT_TOKEN="+v.token,
	)

	if v.caCert != "" {
		cmd.Env = append(
			cmd.Env,
			"VAULT_CACERT="+v.caCert,
			"VAULT_SKIP_VERIFY=true",
		)
	}

	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())
	wait(session)
	return session
}

func (v *vault) init() {
	init := v.Run("init")
	content := string(init.Out.Contents())
	v.key1 = regexp.MustCompile(`Unseal Key 1: (.*)`).FindStringSubmatch(content)[1]
	v.key2 = regexp.MustCompile(`Unseal Key 2: (.*)`).FindStringSubmatch(content)[1]
	v.key3 = regexp.MustCompile(`Unseal Key 3: (.*)`).FindStringSubmatch(content)[1]
	v.token = regexp.MustCompile(`Initial Root Token: (.*)`).FindStringSubmatch(content)[1]
	v.Unseal()
	v.Run("mount", "-path", "concourse/main", "generic")
}

func (v *vault) Unseal() {
	v.Run("unseal", v.key1)
	v.Run("unseal", v.key2)
	v.Run("unseal", v.key3)
}
