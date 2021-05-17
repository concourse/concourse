package integration_test

import (
	"net/http"
	"os/exec"
	"strings"

	"github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Fly CLI", func() {
	Describe("clear-resource-cache", func() {
		var (
			flyCmd *exec.Cmd
			expectedQueryParams []string
			expectedURL         = "/api/v1/teams/main/pipelines/some-pipeline/resources/some-resource/cache"
		)

		Context("when a resource is not specified", func() {
			It("asks the user to specify a resource", func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "clear-resource-cache")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(1))

				Expect(sess.Err).To(gbytes.Say("error: the required flag `" + osFlag("r", "resource") + "' was not specified"))
			})
		})


		Context("when resource and a version are specified", func() {
			JustBeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", expectedURL, strings.Join(expectedQueryParams, "&")),
						ghttp.VerifyJSON(`{"version":{"ref":"fake-ref"}}`),
						ghttp.RespondWithJSONEncoded(http.StatusOK, atc.ClearResourceCacheResponse{CachesRemoved: 1}),
					),
				)
			})

			It("succeeds with one deletion", func() {
				Expect(func() {
					flyCmd = exec.Command(flyPath, "-t", targetName, "clear-resource-cache", "-r", "some-pipeline/some-resource", "-v", "ref:fake-ref")
					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess).Should(gexec.Exit(0))
					Eventually(sess).Should(gbytes.Say("1 caches removed"))
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(2))
			})
		})

		Context("when only a resource is specified", func() {
			Context("when the resource exists", func() {
				JustBeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", expectedURL, strings.Join(expectedQueryParams, "&")),
							ghttp.RespondWithJSONEncoded(http.StatusOK, atc.ClearResourceCacheResponse{CachesRemoved: 1}),
						),
					)
				})

				It("succeeds with one deletion", func() {
					Expect(func() {
						flyCmd = exec.Command(flyPath, "-t", targetName, "clear-resource-cache", "-r", "some-pipeline/some-resource")
						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess).Should(gexec.Exit(0))
						Eventually(sess).Should(gbytes.Say("1 caches removed"))
					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(2))
				})
			})


			Context("when the resource does not exist", func() {
				var (
					flyCmd *exec.Cmd
					expectedQueryParams []string
					expectedURL         = "/api/v1/teams/main/pipelines/some-pipeline/resources/no-existing-resource/cache"
				)

				JustBeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", expectedURL, strings.Join(expectedQueryParams, "&")),
							ghttp.RespondWithJSONEncoded(http.StatusNotFound, ""),
						),
					)
				})

				It("writes that it did not exist", func() {
					Expect(func() {
						flyCmd = exec.Command(flyPath, "-t", targetName, "clear-resource-cache", "-r", "some-pipeline/no-existing-resource")
						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess).Should(gexec.Exit(1))
						Eventually(sess.Err).Should(gbytes.Say("resource not found"))
					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(2))
				})
			})



			Context("and the api returns an unexpected status code", func() {
				JustBeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", expectedURL, strings.Join(expectedQueryParams, "&")),
							ghttp.RespondWith(http.StatusInternalServerError, ""),
						),
					)
				})

				It("writes an error message to stderr", func() {
					Expect(func() {
						flyCmd = exec.Command(flyPath, "-t", targetName, "clear-resource-cache", "-r", "some-pipeline/some-resource")
						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess.Err).Should(gbytes.Say("Unexpected Response"))
						Eventually(sess).Should(gexec.Exit(1))
					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(2))

				})
			})
		})
	})
})
