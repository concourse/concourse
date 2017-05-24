package topgun_test

import (
	"bytes"
	"database/sql"
	"fmt"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Database secrets encryption", func() {
	var dbConn *sql.DB

	Describe("Database secrets are not stored as plaintext from being initially deployed with the encryption key", func() {
		BeforeEach(func() {
			var err error
			dbConn, err = sql.Open("postgres", fmt.Sprintf("postgres://atc:dummy-password@%s:5432/atc?sslmode=disable", atcIP))
			Expect(err).ToNot(HaveOccurred())

			Deploy("deployments/single-vm-with-encryption.yml")
		})

		It("is encrypted into the database", func() {
			By("setting a pipeline that contains secrets")
			fly("set-pipeline", "-n", "-c", "pipelines/secrets.yml", "-p", "pipeline-secrets-test")

			By("creating a team with auth")
			setTeamSession := spawnFlyInteractive(
				bytes.NewBufferString("y\n"),
				"set-team", "--team-name", "new-team", "--github-auth-user", "fakeUser", "--github-auth-client-id", "victorias_secret", "--github-auth-client-secret", "victorias_secret")
			<-setTeamSession.Exited

			By("getting a pg_dump")
			dump := exec.Command("pg_dump", "-U", "atc", "-h", atcIP, "atc")
			dump.Env = append(os.Environ(), "PGPASSWORD=dummy-password")
			dump.Stdin = bytes.NewBufferString("dummy-password\n")
			session, err := gexec.Start(dump, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			<-session.Exited
			Expect(session).To(gexec.Exit(0))
			Expect(session).ToNot(gbytes.Say("victorias_secret"))
			Expect(session).ToNot(gbytes.Say("resource_secret"))
			Expect(session).ToNot(gbytes.Say("resource_type_secret"))
			Expect(session).ToNot(gbytes.Say("job_secret"))
		})
	})

	Describe("Database secrets are not stored as plaintext from being redeployed with the encryption key", func() {
		BeforeEach(func() {
			var err error
			dbConn, err = sql.Open("postgres", fmt.Sprintf("postgres://atc:dummy-password@%s:5432/atc?sslmode=disable", atcIP))
			Expect(err).ToNot(HaveOccurred())

			Deploy("deployments/single-vm.yml")
		})

		It("is encrypted into the database", func() {
			By("setting a pipeline that contains secrets")
			fly("set-pipeline", "-n", "-c", "pipelines/secrets.yml", "-p", "pipeline-secrets-test")

			By("creating a team with auth")
			setTeamSession := spawnFlyInteractive(
				bytes.NewBufferString("y\n"),
				"set-team", "--team-name", "new-team", "--github-auth-user", "fakeUser", "--github-auth-client-id", "victorias_secret", "--github-auth-client-secret", "victorias_secret")
			<-setTeamSession.Exited

			By("getting a pg_dump")
			dump := exec.Command("pg_dump", "-U", "atc", "-h", atcIP, "atc")
			dump.Env = append(os.Environ(), "PGPASSWORD=dummy-password")
			dump.Stdin = bytes.NewBufferString("dummy-password\n")
			session, err := gexec.Start(dump, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			<-session.Exited
			Expect(session).To(gexec.Exit(0))
			Expect(session.Out.Contents()).To(ContainSubstring("victorias_secret"))
			Expect(session.Out.Contents()).To(ContainSubstring("resource_secret"))
			Expect(session.Out.Contents()).To(ContainSubstring("resource_type_secret"))
			Expect(session.Out.Contents()).To(ContainSubstring("job_secret"))

			By("redeployed deployment with the encryption key")
			Deploy("deployments/single-vm-with-encryption.yml")

			By("getting a pg_dump")
			dump = exec.Command("pg_dump", "-U", "atc", "-h", atcIP, "atc")
			dump.Env = append(os.Environ(), "PGPASSWORD=dummy-password")
			dump.Stdin = bytes.NewBufferString("dummy-password\n")
			session, err = gexec.Start(dump, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			<-session.Exited
			Expect(session).To(gexec.Exit(0))
			Expect(session).ToNot(gbytes.Say("victorias_secret"))
			Expect(session).ToNot(gbytes.Say("resource_secret"))
			Expect(session).ToNot(gbytes.Say("resource_type_secret"))
			Expect(session).ToNot(gbytes.Say("job_secret"))
		})
	})
})
