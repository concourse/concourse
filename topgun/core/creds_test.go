package topgun_test

import (
	"github.com/onsi/gomega/gbytes"

	. "github.com/concourse/concourse/topgun"
	. "github.com/concourse/concourse/topgun/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const assertionScript = `#!/bin/sh

test "$SECRET_USERNAME" = "some_username"
test "$SECRET_PASSWORD" = "some_password"
test "$TEAM_SECRET" = "some_team_secret"

test "$MIRRORED_VERSION" = "some_exposed_version_secret"

test "$(cat some-resource/resource_secret)" = "some_resource_secret"
test "$(cat custom-resource/custom_resource_secret)" = "some_resource_secret"
test "$(cat params-in-get/username)" = "get_some_username"
test "$(cat params-in-get/password)" = "get_some_password"
test "$(cat params-in-put/version)" = "some_exposed_version_secret"
test "$(cat params-in-put/username)" = "put_get_some_username"
test "$(cat params-in-put/password)" = "put_get_some_password"

# note: don't assert against canary/canary, since that's used for
# testing that the credential isn't visible in 'get-pipeline'

echo all credentials matched expected values
`

func testCredentialManagement(
	pipelineSetup func(),
	oneOffSetup func(),
) {
	Context("with a pipeline build", func() {
		BeforeEach(func() {
			pipelineSetup()

			By("setting a pipeline that uses vars for secrets")
			Fly.Run("set-pipeline", "-n", "-c", "pipelines/credential-management.yml", "-p", "pipeline-creds-test")

			By("getting the pipeline config")
			session := Fly.Start("get-pipeline", "-p", "pipeline-creds-test")
			<-session.Exited
			Expect(session.ExitCode()).To(Equal(0))
			Expect(string(session.Out.Contents())).ToNot(ContainSubstring("some_canary"))
			Expect(string(session.Out.Contents())).To(ContainSubstring("((resource_type_secret))"))
			Expect(string(session.Out.Contents())).To(ContainSubstring("((resource_secret))"))
			Expect(string(session.Out.Contents())).To(ContainSubstring("((job_secret.username))"))
			Expect(string(session.Out.Contents())).To(ContainSubstring("((job_secret.password))"))
			Expect(string(session.Out.Contents())).To(ContainSubstring("((resource_version))"))
			Expect(string(session.Out.Contents())).To(ContainSubstring("((team_secret))"))

			By("unpausing the pipeline")
			Fly.Run("unpause-pipeline", "-p", "pipeline-creds-test")
		})

		It("parameterizes via Vault and leaves the pipeline uninterpolated", func() {
			By("triggering job")
			watch := Fly.Start("trigger-job", "-w", "-j", "pipeline-creds-test/some-job")
			Wait(watch)
			Expect(watch).To(gbytes.Say("all credentials matched expected values"))

			By("taking a dump")
			session := PgDump()
			Expect(session).ToNot(gbytes.Say("some_resource_type_secret"))
			Expect(session).ToNot(gbytes.Say("some_resource_secret"))
			Expect(session).ToNot(gbytes.Say("some_username"))
			Expect(session).ToNot(gbytes.Say("some_password"))
			Expect(session).ToNot(gbytes.Say("some_team_secret"))

			// versions aren't protected
			Expect(session).To(gbytes.Say("some_exposed_version_secret"))
		})

		Context("when the job's inputs are used for a one-off build", func() {
			It("parameterizes the values using the job's pipeline scope", func() {
				By("triggering job to populate its inputs")
				watch := Fly.Start("trigger-job", "-w", "-j", "pipeline-creds-test/some-job")
				Wait(watch)
				Expect(watch).To(gbytes.Say("all credentials matched expected values"))

				By("executing a task that parameterizes image_resource and uses a pipeline resource with credentials")
				watch = Fly.StartWithEnv(
					[]string{
						"EXPECTED_RESOURCE_SECRET=some_resource_secret",
						"EXPECTED_RESOURCE_VERSION_SECRET=some_exposed_version_secret",
					},
					"execute",
					"-c", "tasks/credential-management-with-job-inputs.yml",
					"-j", "pipeline-creds-test/some-job",
				)
				Wait(watch)
				Expect(watch).To(gbytes.Say("all credentials matched expected values"))

				By("taking a dump")
				session := PgDump()
				Expect(session).ToNot(gbytes.Say("some_resource_secret"))

				// versions aren't protected
				Expect(session).To(gbytes.Say("some_exposed_version_secret"))
			})
		})
	})

	Context("with a one-off build", func() {
		BeforeEach(oneOffSetup)

		It("parameterizes image_resource and params in a task config", func() {
			watch := Fly.StartWithEnv(
				[]string{
					"EXPECTED_TEAM_SECRET=some_team_secret",
					"EXPECTED_RESOURCE_VERSION_SECRET=some_exposed_version_secret",
				},
				"execute", "-c", "tasks/credential-management.yml",
			)
			Wait(watch)
			Expect(watch).To(gbytes.Say("all credentials matched expected values"))

			By("taking a dump")
			session := PgDump()
			Expect(session).ToNot(gbytes.Say("some_team_secret"))

			// versions aren't protected
			Expect(session).To(gbytes.Say("some_exposed_version_secret"))
		})
	})
}
