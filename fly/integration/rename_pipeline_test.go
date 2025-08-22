package integration_test

import (
	"fmt"
	"net/http"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/atc"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("RenamePipeline", func() {
	Context("when team is not specified", func() {
		var (
			expectedURL        string
			expectedStatusCode int
			newName            string
		)

		BeforeEach(func() {
			expectedURL = "/api/v1/teams/main/pipelines/some-pipeline/rename"
			expectedStatusCode = http.StatusNoContent
			newName = "some-new-name"
		})

		JustBeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", expectedURL),
					ghttp.VerifyJSON(fmt.Sprintf(`{"name":%q}`, newName)),
					ghttp.RespondWith(expectedStatusCode, ""),
				),
			)
		})

		Context("when not specifying a pipeline name", func() {
			It("fails and says you should provide a pipeline name", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "rename-pipeline", "-n", "some-new-name")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(1))

				Expect(sess.Err).To(gbytes.Say("error: the required flag `" + osFlag("o", "old-name") + "' was not specified"))
			})
		})

		Context("when specifying a new pipeline name with a '/' character in it", func() {
			It("fails and says '/' characters are not allowed", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "rename-pipeline", "-o", "some-pipeline", "-n", "forbidden/pipelinename")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))

				Expect(sess.Err).To(gbytes.Say("error: invalid_identifier: pipeline: 'forbidden/pipelinename' is not a valid identifier: illegal character '/'"))
			})
		})

		Context("when the pipeline flag is invalid", func() {
			It("fails and print invalid flag error", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "rename-pipeline", "-o", "pipeline/branch:master", "-n", "some-new-name")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))

			})
		})

		Context("when not specifying a new name", func() {
			It("fails and says you should provide a new name for the pipeline", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "rename-pipeline", "-o", "some-pipeline")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(1))

				Expect(sess.Err).To(gbytes.Say("error: the required flag `" + osFlag("n", "new-name") + "' was not specified"))
			})
		})

		Context("when all the inputs are provided", func() {
			It("successfully renames the pipeline to the provided name", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "rename-pipeline", "-o", "some-pipeline", "-n", newName)

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))
				Expect(atcServer.ReceivedRequests()).To(HaveLen(5))
				Expect(sess.Out).To(gbytes.Say(fmt.Sprintf("pipeline successfully renamed to '%s'", newName)))
			})

			Context("when the pipeline is not found", func() {
				BeforeEach(func() {
					expectedStatusCode = http.StatusNotFound
				})

				It("returns an error", func() {
					flyCmd := exec.Command(flyPath, "-t", targetName, "rename-pipeline", "-o", "some-pipeline", "-n", newName)

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess).Should(gexec.Exit(1))
					Expect(atcServer.ReceivedRequests()).To(HaveLen(5))
					Expect(sess.Err).To(gbytes.Say("pipeline 'some-pipeline' not found"))
				})
			})

			Context("when an error occurs", func() {
				BeforeEach(func() {
					expectedStatusCode = http.StatusTeapot
				})

				It("returns an error", func() {
					flyCmd := exec.Command(flyPath, "-t", targetName, "rename-pipeline", "-o", "some-pipeline", "-n", newName)

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess).Should(gexec.Exit(1))
					Expect(atcServer.ReceivedRequests()).To(HaveLen(5))
					Expect(sess.Err).To(gbytes.Say("418 I'm a teapot"))
				})
			})
		})

	})

	Context("when a non-default team is specified", func() {
		var newName = "brandNew"
		var expectedStatusCode = http.StatusNoContent
		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/teams/other-team"),
					ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Team{
						Name: "other-team",
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", "/api/v1/teams/other-team/pipelines/some-pipeline/rename"),
					ghttp.VerifyJSON(fmt.Sprintf(`{"name":%q}`, newName)),
					ghttp.RespondWith(expectedStatusCode, ""),
				),
			)
		})

		It("succeeds and renames the pipeline to the provided name", func() {
			flyCmd := exec.Command(flyPath, "-t", targetName, "rename-pipeline", "-o", "some-pipeline", "-n", newName, "--team", "other-team")

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(1))

		})
	})
})
