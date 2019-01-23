package integration_test

import (
	"fmt"
	"github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/rata"
	"net/http"
	"os/exec"

	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Fly CLI", func() {
	Describe("pin-resource-version", func() {
		var (
			expectedStatus    int
			path              string
			err               error
			teamName          = "main"
			pipelineName      = "pipeline"
			resourceName      = "resource"
			resourceVersionID = "42"
			pipelineResource  = fmt.Sprintf("%s/%s", pipelineName, resourceName)
		)

		BeforeEach(func() {
			path, err = atc.Routes.CreatePathForRoute(atc.PinResourceVersion, rata.Params{
				"pipeline_name":              pipelineName,
				"team_name":                  teamName,
				"resource_name":              resourceName,
				"resource_config_version_id": resourceVersionID,
			})
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", path),
					ghttp.RespondWith(expectedStatus, nil),
				),
			)
		})

		Context("make sure the command exists", func() {
			It("calls the pin-resource-version command", func() {
				flyCmd := exec.Command(flyPath, "pin-resource-version")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)

				Expect(err).ToNot(HaveOccurred())
				Consistently(sess.Err).ShouldNot(gbytes.Say("error: Unknown command"))

				<-sess.Exited
			})
		})

		Context("when the resource is specified", func() {
			Context("when the resource version id is specified", func() {
				Context("when the resource and version id exists", func() {
					BeforeEach(func() {
						expectedStatus = http.StatusOK
					})

					It("pins the resource version", func() {
						Expect(func() {
							flyCmd := exec.Command(flyPath, "-t", targetName, "pin-resource-version", "-r", pipelineResource, "-i", resourceVersionID)

							sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())

							Eventually(sess.Out).Should(gbytes.Say(fmt.Sprintf("pinned '%s' at version id %s\n", pipelineResource, resourceVersionID)))

							<-sess.Exited
							Expect(sess.ExitCode()).To(Equal(0))
						}).To(Change(func() int {
							return len(atcServer.ReceivedRequests())
						}).By(2))
					})
				})

				Context("when the resource or version id does not exist", func() {
					BeforeEach(func() {
						expectedStatus = http.StatusNotFound
					})

					It("fails to pin", func() {
						Expect(func() {
							flyCmd := exec.Command(flyPath, "-t", targetName, "pin-resource-version", "-r", pipelineResource, "-i", resourceVersionID)

							sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())

							Eventually(sess.Err).Should(gbytes.Say(fmt.Sprintf("could not pin '%s' at version %s, make sure the resource and version exists",
								pipelineResource, resourceVersionID)))

							<-sess.Exited
							Expect(sess.ExitCode()).To(Equal(1))
						}).To(Change(func() int {
							return len(atcServer.ReceivedRequests())
						}).By(2))
					})
				})

			})

			Context("when the resource version id is not specified", func() {
				It("errors", func() {
					Expect(func() {
						flyCmd := exec.Command(flyPath, "-t", targetName, "pin-resource-version", "-r", pipelineResource)

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess.Err).Should(gbytes.Say("error:.*-i, --version-id.*not specified"))

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(1))
					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(0))
				})
			})
		})

		Context("when the resource is not specified", func() {
			It("errors", func() {
				Expect(func() {
					flyCmd := exec.Command(flyPath, "-t", targetName, "pin-resource-version")

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess.Err).Should(gbytes.Say("error:.*-r, --resource.*not specified"))

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(1))
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(0))
			})
		})
	})
})
