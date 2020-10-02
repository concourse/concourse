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
	Describe("pin-resource", func() {
		var (
			expectedGetStatus             int
			expectedPutStatus             int
			expectedPutCommentStatus      int
			pinPath, getPath, commentPath string
			err                           error
			teamName                      = "main"
			pipelineName                  = "pipeline"
			resourceName                  = "resource"
			resourceVersionID             = "42"
			pinVersion                    = "some:value"
			pipelineRef                   = atc.PipelineRef{Name: pipelineName, InstanceVars: atc.InstanceVars{"branch": "master"}}
			pipelineResource              = fmt.Sprintf("%s/%s", pipelineRef.String(), resourceName)
			expectedPinVersion            = atc.ResourceVersion{
				ID:      42,
				Version: atc.Version{"some": "value"},
			}
			expectedQueryParams []string
		)

		BeforeEach(func() {
			expectedQueryParams = []string{}
		})

		Context("make sure the command exists", func() {
			It("calls the pin-resource command", func() {
				flyCmd := exec.Command(flyPath, "pin-resource")
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

					pinPath, err = atc.Routes.CreatePathForRoute(atc.PinResourceVersion, rata.Params{
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
							ghttp.RespondWithJSONEncoded(expectedGetStatus, []atc.ResourceVersion{expectedPinVersion}),
						),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("PUT", pinPath, "instance_vars=%7B%22branch%22%3A%22master%22%7D"),
							ghttp.RespondWith(expectedPutStatus, nil),
						),
					)
				})
				Context("when the resource and version exists", func() {
					BeforeEach(func() {
						expectedGetStatus = http.StatusOK
						expectedPutStatus = http.StatusOK
					})

					It("pins the resource version", func() {
						Expect(func() {
							flyCmd := exec.Command(flyPath, "-t", targetName, "pin-resource", "-r", pipelineResource, "-v", pinVersion)

							sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())

							Eventually(sess.Out).Should(gbytes.Say(fmt.Sprintf("pinned '%s' with version {\"some\":\"value\"}\n", pipelineResource)))

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
							flyCmd := exec.Command(flyPath, "-t", targetName, "pin-resource", "-r", pipelineResource, "-v", pinVersion)

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

				Context("when the resource does not exist", func() {
					BeforeEach(func() {
						expectedPutStatus = http.StatusNotFound
						expectedGetStatus = http.StatusOK
					})

					It("fails to pin", func() {
						Expect(func() {
							flyCmd := exec.Command(flyPath, "-t", targetName, "pin-resource", "-r", pipelineResource, "-v", pinVersion)

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

			Context("when pin comment is provided", func() {
				BeforeEach(func() {
					commentPath, err = atc.Routes.CreatePathForRoute(atc.SetPinCommentOnResource, rata.Params{
						"pipeline_name": pipelineName,
						"team_name":     teamName,
						"resource_name": resourceName,
					})
					Expect(err).NotTo(HaveOccurred())
				})

				JustBeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("PUT", commentPath),
							ghttp.VerifyJSONRepresenting(atc.SetPinCommentRequestBody{PinComment: "some pin message"}),
							ghttp.RespondWith(expectedPutCommentStatus, nil),
						),
					)
				})

				Context("when resource is pinned", func() {
					BeforeEach(func() {
						expectedPutCommentStatus = http.StatusOK
					})

					It("save the comment to pin resource", func() {
						Expect(func() {
							flyCmd := exec.Command(flyPath, "-t", targetName, "pin-resource", "-r", pipelineResource, "-c", "some pin message")

							sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())

							Eventually(sess.Out).Should(gbytes.Say(fmt.Sprintf("pin comment 'some pin message' is saved\n")))
							<-sess.Exited
							Expect(sess.ExitCode()).To(Equal(0))
						}).To(Change(func() int {
							return len(atcServer.ReceivedRequests())
						}).By(2))
					})
				})

				Context("when resource is not pinned", func() {
					BeforeEach(func() {
						expectedPutCommentStatus = http.StatusNotFound
					})

					It("shows error", func() {
						Expect(func() {
							flyCmd := exec.Command(flyPath, "-t", targetName, "pin-resource", "-r", pipelineResource, "-c", "some pin message")

							sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())

							Eventually(sess.Err).Should(gbytes.Say(fmt.Sprintf("could not save comment, make sure '%s' is pinned\n", pipelineResource)))
							<-sess.Exited
							Expect(sess.ExitCode()).To(Equal(1))
						}).To(Change(func() int {
							return len(atcServer.ReceivedRequests())
						}).By(2))
					})
				})
			})
		})
	})
})
