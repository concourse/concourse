package integration_test

import (
	"net/http"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/rata"
)

var _ = Describe("Fly CLI", func() {
	Describe("trigger-job", func() {
		Context("when the pipeline and job name are specified", func() {
			var (
				path string
				err  error
			)
			BeforeEach(func() {
				path, err = atc.Routes.CreatePathForRoute(atc.CreateJobBuild, rata.Params{"pipeline_name": "awesome-pipeline", "job_name": "awesome-job"})
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when the pipeline and job exists", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("POST", path),
							ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Build{}),
						),
					)
				})

				It("starts the build", func() {
					Expect(func() {
						flyCmd := exec.Command(flyPath, "-t", targetName, "trigger-job", "-j", "awesome-pipeline/awesome-job")

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess).Should(gbytes.Say(`started 'awesome-pipeline/awesome-job'`))

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))
					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(2))
				})
			})

			Context("when the pipeline/job doesn't exist", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("POST", path),
							ghttp.RespondWith(http.StatusNotFound, nil),
						),
					)
				})

				It("prints an error message", func() {
					Expect(func() {
						flyCmd := exec.Command(flyPath, "-t", targetName, "trigger-job", "-j", "awesome-pipeline/awesome-job")

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess.Err).Should(gbytes.Say(`error: resource not found`))

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(1))
					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(2))
				})
			})
		})

		Context("when the pipeline/job name is not specified", func() {
			It("errors", func() {
				reqsBefore := len(atcServer.ReceivedRequests())
				flyCmd := exec.Command(flyPath, "-t", targetName, "trigger-job")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))
				Expect(atcServer.ReceivedRequests()).To(HaveLen(reqsBefore))
			})
		})
	})
})
