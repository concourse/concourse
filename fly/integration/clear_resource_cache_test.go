package integration_test

import (
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"

	"github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Fly CLI", func() {
	Describe("clear-resource-cache", func() {
		var (
			expectedQueryParams []string
			expectedURL         = "/api/v1/teams/main/pipelines/some-pipeline/resources/some-resource/cache"
			stdin               io.Writer
			args                []string
			sess                *gexec.Session
		)

		BeforeEach(func() {
			stdin = nil
			args = []string{}
		})

		JustBeforeEach(func() {
			var err error

			flyCmd := exec.Command(flyPath, append([]string{"-t", targetName, "clear-resource-cache"}, args...)...)
			stdin, err = flyCmd.StdinPipe()
			Expect(err).NotTo(HaveOccurred())

			sess, err = gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
		})

		yes := func() {
			Eventually(sess).Should(gbytes.Say(`are you sure\? \[yN\]: `))
			fmt.Fprintf(stdin, "y\n")
		}

		no := func() {
			Eventually(sess).Should(gbytes.Say(`are you sure\? \[yN\]: `))
			fmt.Fprintf(stdin, "n\n")
		}

		Context("when a resource is not specified", func() {
			It("asks the user to specify a resource", func() {
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

			BeforeEach(func() {
				args = append(args, "-r", "some-pipeline/some-resource", "-v", "ref:fake-ref")
			})

			It("warns this could be a dangerous command", func() {
				Eventually(sess).Should(gbytes.Say("this will remove the resource cache\\(s\\) for `some-pipeline/some-resource` with version: `{\"ref\":\"fake-ref\"}`"))
			})

			It("succeeds with one deletion", func() {
				Expect(func() {
					yes()
					Eventually(sess).Should(gexec.Exit(0))
					Eventually(sess).Should(gbytes.Say("1 caches removed"))
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(2))
			})

			It("bails out if the user negate the command", func() {
				no()
				Eventually(sess).Should(gbytes.Say(`bailing out`))
				Eventually(sess).Should(gexec.Exit(0))
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

				BeforeEach(func() {
					args = append(args, "-r", "some-pipeline/some-resource")
				})

				It("succeeds with one deletion", func() {
					Expect(func() {
						yes()
						Eventually(sess).Should(gexec.Exit(0))
						Eventually(sess).Should(gbytes.Say("1 caches removed"))
					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(2))
				})
			})

			Context("when the resource does not exist", func() {
				var expectedURL = "/api/v1/teams/main/pipelines/some-pipeline/resources/no-existing-resource/cache"

				JustBeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", expectedURL, strings.Join(expectedQueryParams, "&")),
							ghttp.RespondWithJSONEncoded(http.StatusNotFound, ""),
						),
					)
				})

				BeforeEach(func() {
					args = append(args, "-r", "some-pipeline/no-existing-resource")
				})

				It("writes that it did not exist", func() {
					Expect(func() {
						yes()
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

				BeforeEach(func() {
					args = append(args, "-r", "some-pipeline/some-resource")
				})

				It("writes an error message to stderr", func() {
					Expect(func() {
						yes()
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
