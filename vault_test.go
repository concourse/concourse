package topgun_test

import (
	"bytes"
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
		var vaultAddr string
		var vaultToken string

		vault := func(command string, args ...string) *gexec.Session {
			cmd := exec.Command("vault", append([]string{command}, args...)...)
			cmd.Env = append(os.Environ(), "VAULT_ADDR="+vaultAddr, "VAULT_TOKEN="+vaultToken)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			wait(session)
			return session
		}

		BeforeEach(func() {
			Deploy("deployments/vault.yml")

			vaultInstance := JobInstance("vault")
			vaultAddr = "http://" + vaultInstance.IP + ":8200"

			init := vault("init")
			content := string(init.Out.Contents())
			key1 := regexp.MustCompile(`Unseal Key 1: (.*)`).FindStringSubmatch(content)[1]
			key2 := regexp.MustCompile(`Unseal Key 2: (.*)`).FindStringSubmatch(content)[1]
			key3 := regexp.MustCompile(`Unseal Key 3: (.*)`).FindStringSubmatch(content)[1]

			vaultToken = regexp.MustCompile(`Initial Root Token: (.*)`).FindStringSubmatch(content)[1]

			vault("unseal", key1)
			vault("unseal", key2)
			vault("unseal", key3)

			vault("mount", "-path", "main", "generic")
			vault("write", "main/pipeline-vault-test/resource-type-repository", "value=concourse/time-resource")
			vault("write", "main/pipeline-vault-test/time-resource-interval", "value=10m")
			vault("write", "main/pipeline-vault-test/job-secret", "value=Hello")
			vault("write", "main/pipeline-vault-test/image-resource-repository", "value=busybox")

			Deploy(
				"deployments/vault-with-concourse.yml",
				"-v", "vault-addr="+vaultAddr,
				"-v", "vault-token="+vaultToken,
			)
		})

		It("parameterizes via Vault and leaves the pipeline uninterpolated", func() {
			By("setting a pipeline that contains vault secrets")
			fly("set-pipeline", "-n", "-c", "pipelines/vault.yml", "-p", "pipeline-vault-test")

			By("getting the pipeline config")
			session := getPipeline()
			Expect(string(session.Out.Contents())).ToNot(ContainSubstring("concourse/time-resource"))
			Expect(string(session.Out.Contents())).ToNot(ContainSubstring("10m"))
			Expect(string(session.Out.Contents())).ToNot(ContainSubstring("Hello"))
			Expect(string(session.Out.Contents())).ToNot(ContainSubstring("busybox"))

			By("unpausing the pipeline")
			fly("unpause-pipeline", "-p", "pipeline-vault-test")

			By("triggering job")
			watch := spawnFly("trigger-job", "-w", "-j", "pipeline-vault-test/simple-job")
			wait(watch)
			Expect(watch).To(gbytes.Say("SECRET: Hello"))

			By("taking a dump")
			session = pgDump()
			Expect(session).ToNot(gbytes.Say("concourse/time-resource"))
			Expect(session).ToNot(gbytes.Say("10m"))
			Expect(session).To(gbytes.Say("Hello")) // we do not encrypt build events
			Expect(session).ToNot(gbytes.Say("busybox"))
		})
	})
})
