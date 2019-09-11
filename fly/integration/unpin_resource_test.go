package integration_test

import (
	"fmt"
	"github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/rata"
	"net/http"
	"os/exec"
)

var _ = Describe("Fly CLI", func() {
	Describe("unpin-resource", func() {
		var (
			expectedStatus   int
			path             string
			err              error
			teamName         = "main"
			pipelineName     = "pipeline"
			resourceName     = "resource"
			pipelineResource = fmt.Sprintf("%s/%s", pipelineName, resourceName)
		)

		BeforeEach(func() {
			path, err = atc.Routes.CreatePathForRoute(atc.UnpinResource, rata.Params{
				"pipeline_name": pipelineName,
				"team_name":     teamName,
				"resource_name": resourceName,
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
			It("calls the unpin-resource command", func() {
				flyCmd := exec.Command(flyPath, "unpin-resource")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)

				Expect(err).ToNot(HaveOccurred())
				Consistently(sess.Err).ShouldNot(gbytes.Say("error: Unknown command"))

				<-sess.Exited
			})
		})

		Context("when the resource is specified", func() {
			Context("when the resource exists", func() {
				BeforeEach(func() {
					expectedStatus = http.StatusOK
				})

				It("pins the resource version", func() {
					Expect(func() {
						flyCmd := exec.Command(flyPath, "-t", targetName, "unpin-resource", "-r", pipelineResource)

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess.Out).Should(gbytes.Say(fmt.Sprintf("unpinned '%s'\n", pipelineResource)))

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))
					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(2))
				})
			})

			Context("when the resource does not exist", func() {
				BeforeEach(func() {
					expectedStatus = http.StatusNotFound
				})

				It("fails to unpin", func() {
					Expect(func() {
						flyCmd := exec.Command(flyPath, "-t", targetName, "unpin-resource", "-r", pipelineResource)

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess.Err).Should(gbytes.Say(fmt.Sprintf("could not find resource '%s'", pipelineResource)))

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(1))
					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(2))
				})
			})

		})

		Context("when the resource is not specified", func() {
			It("errors", func() {
				Expect(func() {
					flyCmd := exec.Command(flyPath, "-t", targetName, "unpin-resource")

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
