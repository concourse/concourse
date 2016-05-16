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
	BeforeEach(func() {
		expectedBuild := atc.Build{
			ID:      23,
			Name:    "42",
			Status:  "running",
			JobName: "myjob",
			URL:     "/pipelines/my-pipeline/jobs/my-job/builds/42",
			APIURL:  "api/v1/builds/123",
		}

		expectedJobBuildURL := "/api/v1/teams/main/pipelines/my-pipeline/jobs/my-job/builds/42"
		expectedAbortURL := "/api/v1/builds/23/abort"

		atcServer.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", expectedJobBuildURL),
				ghttp.RespondWithJSONEncoded(http.StatusOK, expectedBuild),
			),
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", expectedAbortURL),
				ghttp.RespondWith(http.StatusNoContent, ""),
			),
		)
	})

	Context("when the job name is not specified", func() {
		It("asks the user to specifiy a job name", func() {
			flyCmd := exec.Command(flyPath, "-t", targetName, "abort-build", "-b", "some-build-name")

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(1))

			Expect(sess.Err).To(gbytes.Say("error: the required flag `" + osFlag("j", "job") + "' was not specified"))
		})
	})

	Context("when the build name is not specified", func() {
		It("asks the user to specifiy a build name", func() {
			flyCmd := exec.Command(flyPath, "-t", targetName, "abort-build", "-j", "some-pipeline-name/some-job-name")

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(1))

			Expect(sess.Err).To(gbytes.Say("error: the required flag `" + osFlag("b", "build") + "' was not specified"))
		})
	})

	Context("when the pipeline/build exists", func() {
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

	Context("when getting the job build fails", func() {
		BeforeEach(func() {
			expectedJobBuildURL := "/api/v1/teams/main/pipelines/my-pipeline/jobs/my-job/builds/42"

			atcServer.SetHandler(3, ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", expectedJobBuildURL),
				ghttp.RespondWith(http.StatusInternalServerError, "{}"),
			))
		})

		It("returns a helpful error message", func() {
			Expect(func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "abort-build", "-j", "my-pipeline/my-job", "-b", "42")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(1))

				Expect(sess.Err).To(gbytes.Say("error: failed to get job build"))
			}).To(Change(func() int {
				return len(atcServer.ReceivedRequests())
			}).By(2))
		})
	})

	Context("when the build or pipeline does not exist", func() {
		BeforeEach(func() {
			expectedJobBuildURL := "/api/v1/teams/main/pipelines/my-pipeline/jobs/my-job/builds/42"

			atcServer.SetHandler(3, ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", expectedJobBuildURL),
				ghttp.RespondWith(http.StatusNotFound, "{}"),
			))
		})

		It("returns a helpful error message", func() {
			Expect(func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "abort-build", "-j", "my-pipeline/my-job", "-b", "42")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(1))
				Expect(sess.Err).To(gbytes.Say("error: job build does not exist"))
			}).To(Change(func() int {
				return len(atcServer.ReceivedRequests())
			}).By(2))
		})
	})

	Context("when the build abort fails", func() {
		BeforeEach(func() {
			expectedAbortURL := "/api/v1/builds/23/abort"

			atcServer.SetHandler(4, ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", expectedAbortURL),
				ghttp.RespondWith(http.StatusTeapot, ""),
			),
			)
		})

		It("returns a helpful error message", func() {
			Expect(func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "abort-build", "-j", "my-pipeline/my-job", "-b", "42")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(1))

				Expect(sess.Err).To(gbytes.Say("error: failed to abort build"))
			}).To(Change(func() int {
				return len(atcServer.ReceivedRequests())
			}).By(3))
		})
	})
})
