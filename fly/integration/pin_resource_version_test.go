package integration_test

import (
	"fmt"
	"net/http"
	"os/exec"

	"github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/rata"

	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Fly CLI", func() {
	Describe("pin-resource-version", func() {
		var (
			expectedGetStatus         int
			expectedPutStatus         int
			pinPath                   string
			getPath                   string
			err                       error
			teamName                  = "main"
			pipelineName              = "pipeline"
			resourceName              = "resource"
			resourceVersionID         = "42"
			resourceVersionJsonString = `{"some":"value"}`
			pipelineResource          = fmt.Sprintf("%s/%s", pipelineName, resourceName)
			expectedPinVersion        = atc.ResourceVersion{
				ID:      42,
				Version: atc.Version{"some": "value"},
			}
		)

		BeforeEach(func() {
			getPath, err = atc.Routes.CreatePathForRoute(atc.ListResourceVersions, rata.Params{
				"pipeline_name": pipelineName,
				"team_name":     teamName,
				"resource_name": resourceName,
			})
			Expect(err).NotTo(HaveOccurred())

			pinPath, err = atc.Routes.CreatePathForRoute(atc.PinResourceVersion, rata.Params{
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
					ghttp.VerifyRequest("GET", getPath, "filter=some:value"),
					ghttp.RespondWithJSONEncoded(expectedGetStatus, []atc.ResourceVersion{expectedPinVersion}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", pinPath),
					ghttp.RespondWith(expectedPutStatus, nil),
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
			Context("when the resource version json string is specified", func() {
				Context("when the resource and version exists", func() {
					BeforeEach(func() {
						expectedGetStatus = http.StatusOK
						expectedPutStatus = http.StatusOK
					})

					It("pins the resource version", func() {
						Expect(func() {
							flyCmd := exec.Command(flyPath, "-t", targetName, "pin-resource-version", "-r", pipelineResource, "-v", resourceVersionJsonString)

							sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())

							Eventually(sess.Out.Contents).Should(ContainSubstring(fmt.Sprintf("pinned '%s' at version id %d with %+v\n", pipelineResource, expectedPinVersion.ID, expectedPinVersion.Version)))

							<-sess.Exited
							Expect(sess.ExitCode()).To(Equal(0))
						}).To(Change(func() int {
							return len(atcServer.ReceivedRequests())
						}).By(3))
					})
				})

				Context("when the versions does not exist", func() {
					BeforeEach(func() {
						expectedGetStatus = http.StatusNotFound
					})

					It("errors", func() {
						Expect(func() {
							flyCmd := exec.Command(flyPath, "-t", targetName, "pin-resource-version", "-r", pipelineResource, "-v", resourceVersionJsonString)

							sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())

							Eventually(sess.Err).Should(gbytes.Say(fmt.Sprintf("could not find version matching %s", resourceVersionJsonString)))

							<-sess.Exited
							Expect(sess.ExitCode()).To(Equal(1))
						}).To(Change(func() int {
							return len(atcServer.ReceivedRequests())
						}).By(2))
					})
				})

				Context("when the resource does not exist", func() {
					BeforeEach(func() {
						expectedPutStatus = http.StatusNotFound
						expectedGetStatus = http.StatusOK
					})

					It("fails to pin", func() {
						Expect(func() {
							flyCmd := exec.Command(flyPath, "-t", targetName, "pin-resource-version", "-r", pipelineResource, "-v", resourceVersionJsonString)

							sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())

							Eventually(sess.Err).Should(gbytes.Say(fmt.Sprintf("could not pin '%s', make sure the resource exists", pipelineResource)))

							<-sess.Exited
							Expect(sess.ExitCode()).To(Equal(1))
						}).To(Change(func() int {
							return len(atcServer.ReceivedRequests())
						}).By(3))
					})
				})
			})

			Context("when the resource version is not specified", func() {
				It("errors", func() {
					Expect(func() {
						flyCmd := exec.Command(flyPath, "-t", targetName, "pin-resource-version", "-r", pipelineResource)

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess.Err).Should(gbytes.Say("error:.*-v, --version.*not specified"))

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
