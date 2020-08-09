package integration_test

import (
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	"github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/rata"

	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Fly CLI", func() {
	Describe("enable-resource-version", func() {
		var (
			expectedGetStatus   int
			expectedPutStatus   int
			enablePath, getPath string
			err                 error
			teamName            = "main"
			pipelineName        = "pipeline"
			resourceName        = "resource"
			resourceVersionID   = "42"
			enableVersion       = "some:value"
			pipelineRef         = atc.PipelineRef{Name: pipelineName, InstanceVars: atc.InstanceVars{"branch": "master"}}
			pipelineResource    = fmt.Sprintf("%s/%s", pipelineRef.String(), resourceName)
			expectedVersion     = atc.ResourceVersion{
				ID:      42,
				Version: atc.Version{"some": "value"},
				Enabled: false,
			}
			expectedQueryParams []string
		)

		BeforeEach(func() {
			expectedQueryParams = []string{}
		})

		Context("make sure the command exists", func() {
			It("calls the enable-resource-version command", func() {
				flyCmd := exec.Command(flyPath, "enable-resource-version")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)

				Expect(err).ToNot(HaveOccurred())
				Consistently(sess.Err).ShouldNot(gbytes.Say("error: Unknown command"))

				<-sess.Exited
			})
		})

		Context("when the resource is specified", func() {
			BeforeEach(func() {
				expectedQueryParams = append(expectedQueryParams, "instance_vars=%7B%22branch%22%3A%22master%22%7D")
			})

			Context("when the resource version json string is specified", func() {
				BeforeEach(func() {
					getPath, err = atc.Routes.CreatePathForRoute(atc.ListResourceVersions, rata.Params{
						"pipeline_name": pipelineName,
						"team_name":     teamName,
						"resource_name": resourceName,
					})
					Expect(err).NotTo(HaveOccurred())

					enablePath, err = atc.Routes.CreatePathForRoute(atc.EnableResourceVersion, rata.Params{
						"pipeline_name":              pipelineName,
						"team_name":                  teamName,
						"resource_name":              resourceName,
						"resource_config_version_id": resourceVersionID,
					})
					Expect(err).NotTo(HaveOccurred())

					expectedQueryParams = append(expectedQueryParams, "filter=some:value")
				})

				JustBeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", getPath, strings.Join(expectedQueryParams, "&")),
							ghttp.RespondWithJSONEncoded(expectedGetStatus, []atc.ResourceVersion{expectedVersion}),
						),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("PUT", enablePath, "instance_vars=%7B%22branch%22%3A%22master%22%7D"),
							ghttp.RespondWith(expectedPutStatus, nil),
						),
					)
				})

				Context("when the resource and version exists", func() {
					BeforeEach(func() {
						expectedGetStatus = http.StatusOK
						expectedPutStatus = http.StatusOK
						expectedVersion.Enabled = false
					})

					It("enables the resource version", func() {
						Expect(func() {
							flyCmd := exec.Command(flyPath, "-t", targetName, "enable-resource-version", "-r", pipelineResource, "-v", enableVersion)

							sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())

							Eventually(sess.Out).Should(gbytes.Say(fmt.Sprintf("enabled '%s' with version {\"some\":\"value\"}\n", pipelineResource)))

							<-sess.Exited
							Expect(sess.ExitCode()).To(Equal(0))
						}).To(Change(func() int {
							return len(atcServer.ReceivedRequests())
						}).By(3))
					})
				})

				Context("when the resource does not exist", func() {
					BeforeEach(func() {
						expectedGetStatus = http.StatusNotFound
					})

					It("errors", func() {
						Expect(func() {
							flyCmd := exec.Command(flyPath, "-t", targetName, "enable-resource-version", "-r", pipelineResource, "-v", enableVersion)

							sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())

							Eventually(sess.Err).Should(gbytes.Say(fmt.Sprintf("could not find version matching {\"some\":\"value\"}\n")))

							<-sess.Exited
							Expect(sess.ExitCode()).To(Equal(1))
						}).To(Change(func() int {
							return len(atcServer.ReceivedRequests())
						}).By(2))
					})
				})

				Context("when the resource version does not exist", func() {
					BeforeEach(func() {
						expectedPutStatus = http.StatusNotFound
						expectedGetStatus = http.StatusOK
						expectedVersion.Enabled = false
					})

					It("fails to enable", func() {
						Expect(func() {
							flyCmd := exec.Command(flyPath, "-t", targetName, "enable-resource-version", "-r", pipelineResource, "-v", enableVersion)

							sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())

							Eventually(sess.Err).Should(gbytes.Say(fmt.Sprintf("could not enable '%s', make sure the resource version exists", pipelineResource)))

							<-sess.Exited
							Expect(sess.ExitCode()).To(Equal(1))
						}).To(Change(func() int {
							return len(atcServer.ReceivedRequests())
						}).By(3))
					})
				})

				Context("when the resource version is already enabled", func() {
					BeforeEach(func() {
						expectedGetStatus = http.StatusOK
						expectedVersion.Enabled = true
					})

					It("returns successfully without calling api", func() {
						Expect(func() {
							flyCmd := exec.Command(flyPath, "-t", targetName, "enable-resource-version", "-r", pipelineResource, "-v", enableVersion)

							sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())

							Eventually(sess.Out).Should(gbytes.Say(fmt.Sprintf("enabled '%s' with version {\"some\":\"value\"}\n", pipelineResource)))

							<-sess.Exited
							Expect(sess.ExitCode()).To(Equal(0))

							for _, request := range atcServer.ReceivedRequests() {
								Expect(request.RequestURI).NotTo(Equal(enablePath))
							}
						}).To(Change(func() int {
							return len(atcServer.ReceivedRequests())
						}).By(2))
					})
				})
			})
		})
	})
})
