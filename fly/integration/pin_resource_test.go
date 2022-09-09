package integration_test

import (
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	"github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/rata"

	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Fly CLI", func() {
	Describe("pin-resource", func() {
		var (
			listVersionsStatus                            int
			pinVersionStatus                              int
			saveCommentStatus                             int
			pinVersionPath, listVersionsPath, commentPath string
			err                                           error
			teamName                                      = "main"
			pipelineName                                  = "pipeline"
			resourceName                                  = "resource"
			resourceVersionID                             = "42"
			pinVersion                                    = "some:value"
			pipelineRef                                   = atc.PipelineRef{Name: pipelineName, InstanceVars: atc.InstanceVars{"branch": "master"}}
			pipelineResource                              = fmt.Sprintf("%s/%s", pipelineRef.String(), resourceName)
			versionToPin                                  = atc.ResourceVersion{
				ID:      42,
				Version: atc.Version{"some": "value"},
			}
			expectedQueryParams []string
		)

		BeforeEach(func() {
			listVersionsPath, err = atc.Routes.CreatePathForRoute(atc.ListResourceVersions, rata.Params{
				"pipeline_name": pipelineName,
				"team_name":     teamName,
				"resource_name": resourceName,
			})
			Expect(err).NotTo(HaveOccurred())
			pinVersionPath, err = atc.Routes.CreatePathForRoute(atc.PinResourceVersion, rata.Params{
				"pipeline_name":              pipelineName,
				"team_name":                  teamName,
				"resource_name":              resourceName,
				"resource_config_version_id": resourceVersionID,
			})
			Expect(err).NotTo(HaveOccurred())
			commentPath, err = atc.Routes.CreatePathForRoute(atc.SetPinCommentOnResource, rata.Params{
				"pipeline_name": pipelineName,
				"team_name":     teamName,
				"resource_name": resourceName,
			})
			Expect(err).NotTo(HaveOccurred())
			expectedQueryParams = []string{
				"vars.branch=%22master%22",
				"filter=some:value",
			}
		})

		It("is a subcommand", func() {
			flyCmd := exec.Command(flyPath, "pin-resource")
			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)

			Expect(err).ToNot(HaveOccurred())
			Consistently(sess.Err).ShouldNot(gbytes.Say("error: Unknown command"))

			<-sess.Exited
		})

		It("asks the user to specify a version when no version or comment are specified", func() {
			flyCmd := exec.Command(flyPath, "-t", targetName, "pin-resource", "-r", pipelineResource)
			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(1))
			Expect(sess.Err).To(gbytes.Say("error: the required flag `" + osFlag("v", "version") + "' was not specified"))
		})

		Context("when a version is specified", func() {
			JustBeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", listVersionsPath, strings.Join(expectedQueryParams, "&")),
						ghttp.RespondWithJSONEncoded(listVersionsStatus, []atc.ResourceVersion{versionToPin}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", pinVersionPath, "vars.branch=%22master%22"),
						ghttp.RespondWith(pinVersionStatus, nil),
					),
				)
			})

			Context("when the resource and versions exist and pinning succeeds", func() {
				BeforeEach(func() {
					listVersionsStatus = http.StatusOK
					pinVersionStatus = http.StatusOK
				})

				It("pins the resource version", func() {
					flyCmd := exec.Command(flyPath, "-t", targetName, "pin-resource", "-r", pipelineResource, "-v", pinVersion)

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess.Out).Should(gbytes.Say(fmt.Sprintf("pinned '%s' with version {\"some\":\"value\"}\n", pipelineResource)))

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(0))
				})

			})

			Context("when the resource or version cannot be found", func() {
				BeforeEach(func() {
					listVersionsStatus = http.StatusNotFound
				})

				It("errors", func() {
					flyCmd := exec.Command(flyPath, "-t", targetName, "pin-resource", "-r", pipelineResource, "-v", pinVersion)

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess.Err).Should(gbytes.Say(fmt.Sprintf("could not find version matching {\"some\":\"value\"}\n")))

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(1))
				})
			})

			Context("when the resource disappears before pinning", func() {
				BeforeEach(func() {
					listVersionsStatus = http.StatusOK
					pinVersionStatus = http.StatusNotFound
				})

				It("fails to pin", func() {
					flyCmd := exec.Command(flyPath, "-t", targetName, "pin-resource", "-r", pipelineResource, "-v", pinVersion)

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess.Err).Should(gbytes.Say(fmt.Sprintf("could not pin '%s', make sure the resource exists", pipelineResource)))

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(1))
				})
			})
		})

		Context("when version and comment are provided", func() {
			var sess *gexec.Session
			JustBeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", listVersionsPath, strings.Join(expectedQueryParams, "&")),
						ghttp.RespondWithJSONEncoded(listVersionsStatus, []atc.ResourceVersion{versionToPin}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", pinVersionPath, "vars.branch=%22master%22"),
						ghttp.RespondWith(pinVersionStatus, nil),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", commentPath),
						ghttp.VerifyJSONRepresenting(atc.SetPinCommentRequestBody{PinComment: "some pin message"}),
						ghttp.RespondWith(saveCommentStatus, nil),
					),
				)

				var err error
				flyCmd := exec.Command(flyPath, "-t", targetName, "pin-resource", "-r", pipelineResource, "-c", "some pin message", "-v", pinVersion)
				sess, err = gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when the resource and versions exist, pinning succeeds and saving the comment succeeds", func() {
				BeforeEach(func() {
					listVersionsStatus = http.StatusOK
					pinVersionStatus = http.StatusOK
					saveCommentStatus = http.StatusOK
				})

				It("saves the pin comment", func() {
					Eventually(sess.Out).Should(gbytes.Say(fmt.Sprintf("pin comment 'some pin message' is saved\n")))
					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(0))
				})
			})

			Context("when the resource or versions cannot be found", func() {
				BeforeEach(func() {
					listVersionsStatus = http.StatusNotFound
					pinVersionStatus = http.StatusOK
					saveCommentStatus = http.StatusOK
				})

				It("errors", func() {
					Eventually(sess.Err).Should(gbytes.Say(fmt.Sprintf("could not find version matching")))
					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(1))
				})
			})

			Context("when the resource disappears before pinning", func() {
				BeforeEach(func() {
					listVersionsStatus = http.StatusOK
					pinVersionStatus = http.StatusNotFound
					saveCommentStatus = http.StatusOK
				})

				It("errors", func() {
					Eventually(sess.Err).Should(gbytes.Say(fmt.Sprintf("could not pin '%s', make sure the resource exists", pipelineResource)))
					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(1))
				})
			})
		})

		Context("when comment is provided without version and saving the comment fails", func() {
			BeforeEach(func() {
				saveCommentStatus = http.StatusNotFound
			})

			JustBeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", commentPath),
						ghttp.VerifyJSONRepresenting(atc.SetPinCommentRequestBody{PinComment: "some pin message"}),
						ghttp.RespondWith(saveCommentStatus, nil),
					),
				)
			})

			It("errors", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "pin-resource", "-r", pipelineResource, "-c", "some pin message")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess.Err).Should(gbytes.Say(fmt.Sprintf("could not save comment, make sure '%s' is pinned", pipelineResource)))
				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))
			})
		})
	})
})
