package topgun_test

import (
	"bytes"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/concourse/concourse/topgun/common"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Database secrets encryption", func() {
	configurePipelineAndTeamAndTriggerJob := func() {
		By("setting a pipeline that contains secrets")
		Fly.Run("set-pipeline", "-n", "-c", "pipelines/secrets.yml", "-p", "pipeline-secrets-test")
		Fly.Run("unpause-pipeline", "-p", "pipeline-secrets-test")

		By("creating a team with auth")
		setTeamSession := Fly.SpawnInteractive(
			bytes.NewBufferString("y\n"),
			"set-team",
			"--team-name", "victoria",
			"--github-user", "victorias_id",
			"--github-org", "victorias_secret_org",
		)
		<-setTeamSession.Exited

		buildSession := Fly.Start("trigger-job", "-w", "-j", "pipeline-secrets-test/simple-job")
		<-buildSession.Exited
		Expect(buildSession.ExitCode()).To(Equal(0))
	}

	getPipeline := func() *gexec.Session {
		session := Fly.Start("get-pipeline", "-p", "pipeline-secrets-test")
		<-session.Exited
		Expect(session.ExitCode()).To(Equal(0))
		return session
	}

	Describe("A deployment with encryption enabled immediately", func() {
		BeforeEach(func() {
			Deploy("deployments/concourse.yml", "-o", "operations/encryption.yml")
		})

		It("encrypts pipeline credentials", func() {
			configurePipelineAndTeamAndTriggerJob()

			By("taking a dump")
			session := PgDump()
			Expect(session).ToNot(gbytes.Say("resource_secret"))
			Expect(session).ToNot(gbytes.Say("resource_type_secret"))
			Expect(session).ToNot(gbytes.Say("job_secret"))
		})
	})

	Describe("A deployment with encryption initially not configured", func() {
		BeforeEach(func() {
			Deploy("deployments/concourse.yml")
		})

		Context("with credentials in plaintext", func() {
			BeforeEach(func() {
				configurePipelineAndTeamAndTriggerJob()

				By("taking a dump")
				session := PgDump()
				Expect(string(session.Out.Contents())).To(ContainSubstring("resource_secret"))
				Expect(string(session.Out.Contents())).To(ContainSubstring("resource_type_secret"))
				Expect(string(session.Out.Contents())).To(ContainSubstring("job_secret"))
			})

			Context("when redeployed with encryption enabled", func() {
				BeforeEach(func() {
					Deploy("deployments/concourse.yml", "-o", "operations/encryption.yml")
				})

				It("encrypts pipeline credentials", func() {
					By("taking a dump")
					session := PgDump()
					Expect(session).ToNot(gbytes.Say("resource_secret"))
					Expect(session).ToNot(gbytes.Say("concourse/time-resource"))
					Expect(session).ToNot(gbytes.Say("job_secret"))

					By("getting the pipeline config")
					session = getPipeline()
					Expect(string(session.Out.Contents())).To(ContainSubstring("resource_secret"))
					Expect(string(session.Out.Contents())).To(ContainSubstring("resource_type_secret"))
					Expect(string(session.Out.Contents())).To(ContainSubstring("job_secret"))
					Expect(string(session.Out.Contents())).To(ContainSubstring("image_resource_secret"))
				})

				Context("when the encryption key is rotated", func() {
					BeforeEach(func() {
						Deploy("deployments/concourse.yml", "-o", "operations/encryption-rotated.yml")
					})

					It("can still get and set pipelines", func() {
						By("taking a dump")
						session := PgDump()
						Expect(session).ToNot(gbytes.Say("resource_secret"))
						Expect(session).ToNot(gbytes.Say("concourse/time-resource"))
						Expect(session).ToNot(gbytes.Say("job_secret"))

						By("getting the pipeline config")
						session = getPipeline()
						Expect(string(session.Out.Contents())).To(ContainSubstring("resource_secret"))
						Expect(string(session.Out.Contents())).To(ContainSubstring("resource_type_secret"))
						Expect(string(session.Out.Contents())).To(ContainSubstring("job_secret"))
						Expect(string(session.Out.Contents())).To(ContainSubstring("image_resource_secret"))

						By("setting the pipeline again")
						Fly.Run("set-pipeline", "-n", "-c", "pipelines/secrets.yml", "-p", "pipeline-secrets-test")

						By("getting the pipeline config again")
						session = getPipeline()
						Expect(string(session.Out.Contents())).To(ContainSubstring("resource_secret"))
						Expect(string(session.Out.Contents())).To(ContainSubstring("resource_type_secret"))
						Expect(string(session.Out.Contents())).To(ContainSubstring("job_secret"))
						Expect(string(session.Out.Contents())).To(ContainSubstring("image_resource_secret"))
					})
				})

				Context("when an old key is given but all the data is already using the new key", func() {
					BeforeEach(func() {
						Deploy("deployments/concourse.yml", "-o", "operations/encryption-already-rotated.yml")
					})

					It("can still get and set pipelines", func() {
						By("taking a dump")
						session := PgDump()
						Expect(session).ToNot(gbytes.Say("resource_secret"))
						Expect(session).ToNot(gbytes.Say("concourse/time-resource"))
						Expect(session).ToNot(gbytes.Say("job_secret"))

						By("getting the pipeline config")
						session = getPipeline()
						Expect(string(session.Out.Contents())).To(ContainSubstring("resource_secret"))
						Expect(string(session.Out.Contents())).To(ContainSubstring("resource_type_secret"))
						Expect(string(session.Out.Contents())).To(ContainSubstring("job_secret"))
						Expect(string(session.Out.Contents())).To(ContainSubstring("image_resource_secret"))

						By("setting the pipeline again")
						Fly.Run("set-pipeline", "-n", "-c", "pipelines/secrets.yml", "-p", "pipeline-secrets-test")

						By("getting the pipeline config again")
						session = getPipeline()
						Expect(string(session.Out.Contents())).To(ContainSubstring("resource_secret"))
						Expect(string(session.Out.Contents())).To(ContainSubstring("resource_type_secret"))
						Expect(string(session.Out.Contents())).To(ContainSubstring("job_secret"))
						Expect(string(session.Out.Contents())).To(ContainSubstring("image_resource_secret"))
					})
				})

				Context("when an old key and new key are both given that do not match the key in use", func() {
					var deploy *gexec.Session
					var boshLogs *gexec.Session

					BeforeEach(func() {
						boshLogs = SpawnBosh("logs", "-f")

						deploy = StartDeploy("deployments/concourse.yml", "-o", "operations/encryption-bogus.yml")
						<-deploy.Exited
						Expect(deploy.ExitCode()).To(Equal(1))
					})

					AfterEach(func() {
						boshLogs.Signal(os.Interrupt)
						<-boshLogs.Exited
					})

					AfterEach(func() {
						Deploy("deployments/concourse.yml", "-o", "operations/encryption.yml")
					})

					It("fails to deploy with a useful message", func() {
						Expect(deploy).To(gbytes.Say("Review logs for failed jobs: web"))
						Expect(boshLogs).To(gbytes.Say("row encrypted with neither old nor new key"))
					})
				})

				Context("when the encryption key is removed", func() {
					BeforeEach(func() {
						Deploy("deployments/concourse.yml", "-o", "operations/encryption-removed.yml")
					})

					It("decrypts pipeline credentials", func() {
						By("taking a dump")
						session := PgDump()
						Expect(string(session.Out.Contents())).To(ContainSubstring("resource_secret"))
						Expect(string(session.Out.Contents())).To(ContainSubstring("resource_type_secret"))
						Expect(string(session.Out.Contents())).To(ContainSubstring("job_secret"))

						By("getting the pipeline config")
						session = getPipeline()
						Expect(string(session.Out.Contents())).To(ContainSubstring("resource_secret"))
						Expect(string(session.Out.Contents())).To(ContainSubstring("resource_type_secret"))
						Expect(string(session.Out.Contents())).To(ContainSubstring("job_secret"))
						Expect(string(session.Out.Contents())).To(ContainSubstring("image_resource_secret"))
					})
				})
			})
		})
	})
})
