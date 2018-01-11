package integration_test

import (
	"net/http"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"

	"github.com/concourse/atc"
)

var _ = Describe("AbortBuild", func() {
	var expectedAbortURL = "/api/v1/builds/23/abort"

	var expectedBuild = atc.Build{
		ID:      23,
		Name:    "42",
		Status:  "running",
		JobName: "myjob",
		APIURL:  "api/v1/builds/123",
	}

	Context("when the job name is not specified", func() {
		Context("and the build id is specified", func() {
			BeforeEach(func() {
				expectedURL := "/api/v1/builds/23"

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedBuild),
					),

					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", expectedAbortURL),
						ghttp.RespondWith(http.StatusNoContent, ""),
					),
				)
			})

			It("aborts the build", func() {
				Expect(func() {
					flyCmd := exec.Command(flyPath, "-t", targetName, "abort-build", "-b", "23")

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess).Should(gexec.Exit(0))

					Expect(sess.Out).To(gbytes.Say("build successfully aborted"))
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(3))
			})
		})

		Context("and the build id does not exist", func() {
			BeforeEach(func() {
				expectedURL := "/api/v1/builds/42"

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWith(http.StatusNotFound, ""),
					),
				)
			})

			It("asks the user to specify a build id", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "abort-build", "-b", "42")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(1))

				Expect(sess.Err).To(gbytes.Say("error: build does not exist"))
			})
		})

		Context("and the build id is not specified", func() {
			It("asks the user to specify a build id", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "abort-build")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(1))

				Expect(sess.Err).To(gbytes.Say("error: the required flag `" + osFlag("b", "build") + "' was not specified"))
			})
		})
	})

	Context("when the pipeline/build exists", func() {
		Context("and the build name is specified", func() {
			BeforeEach(func() {
				expectedURL := "/api/v1/teams/main/pipelines/my-pipeline/jobs/my-job/builds/42"

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedBuild),
					),

					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", expectedAbortURL),
						ghttp.RespondWith(http.StatusNoContent, ""),
					),
				)
			})

			It("aborts the build", func() {
				Expect(func() {
					flyCmd := exec.Command(flyPath, "-t", targetName, "abort-build", "-j", "my-pipeline/my-job", "-b", "42")

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess).Should(gexec.Exit(0))

					Expect(sess.Out).To(gbytes.Say("build successfully aborted"))
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(3))
			})
		})

		Context("and the build number does not exist", func() {
			BeforeEach(func() {
				expectedURL := "/api/v1/teams/main/pipelines/my-pipeline/jobs/my-job/builds/23"

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWith(http.StatusNotFound, ""),
					),
				)
			})

			It("asks the user to specify a build name", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "abort-build", "-j", "my-pipeline/my-job", "-b", "23")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(1))

				Expect(sess.Err).To(gbytes.Say("error: build does not exist"))
			})
		})

		Context("and the build name is not specified", func() {
			It("asks the user to specify a build name", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "abort-build", "-j", "some-pipeline-name/some-job-name")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(1))

				Expect(sess.Err).To(gbytes.Say("error: the required flag `" + osFlag("b", "build") + "' was not specified"))
			})
		})
	})

	Context("when the build or pipeline does not exist", func() {
		BeforeEach(func() {
			expectedJobBuildURL := "/api/v1/teams/main/pipelines/my-pipeline/jobs/my-job/builds/42"

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedJobBuildURL),
					ghttp.RespondWith(http.StatusNotFound, "{}"),
				),
			)
		})

		It("returns a helpful error message", func() {
			Expect(func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "abort-build", "-j", "my-pipeline/my-job", "-b", "42")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(1))
				Expect(sess.Err).To(gbytes.Say("error: build does not exist"))
			}).To(Change(func() int {
				return len(atcServer.ReceivedRequests())
			}).By(2))
		})
	})
})
