package integration_test

import (
	"fmt"
	"net/http"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Fly CLI", func() {
	Describe("Unpause Job", func() {
		var (
			flyCmd       *exec.Cmd
			pipelineName string
			jobName      string
			fullJobName  string
			apiPath      string
		)

		BeforeEach(func() {
			pipelineName = "pipeline"
			jobName = "job-name-potato"
			fullJobName = fmt.Sprintf("%s/%s", pipelineName, jobName)
			apiPath = fmt.Sprintf("/api/v1/teams/main/pipelines/%s/jobs/%s/unpause", pipelineName, jobName)

			flyCmd = exec.Command(flyPath, "-t", "some-target", "unpause-job", "-j", fullJobName)
		})

		Context("when the job flag is provided", func() {
			Context("when user owns the same team as the given pipeline", func() {
				Context("user is currently on the same team as the given job", func() {
					BeforeEach(func() {
						adminAtcServer.AppendHandlers(
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("PUT", apiPath),
								ghttp.RespondWith(http.StatusOK, nil),
							),
						)
					})

					It("successfully unpauses the job", func() {
						Expect(func() {
							sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())

							<-sess.Exited
							Expect(sess.ExitCode()).To(Equal(0))
							Eventually(sess).Should(gbytes.Say(fmt.Sprintf("unpaused '%s'\n", jobName)))
						}).To(Change(func() int {
							return len(adminAtcServer.ReceivedRequests())
						}).By(2))
					})
				})

				Context("user is NOT currently on the same team as the given job", func() {
					BeforeEach(func() {
						apiPath = fmt.Sprintf("/api/v1/teams/other-team/pipelines/%s/jobs/%s/unpause", pipelineName, jobName)

						adminAtcServer.AppendHandlers(
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("PUT", apiPath),
								ghttp.RespondWith(http.StatusOK, nil),
							),
						)
					})

					It("successfully unpauses the job", func() {
						Expect(func() {
							flyCmd = exec.Command(flyPath, "-t", "some-target", "unpause-job", "-j", fullJobName, "--team", "other-team")
							sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())
							<-sess.Exited
							Expect(sess.ExitCode()).To(Equal(0))
							Eventually(sess).Should(gbytes.Say(fmt.Sprintf("unpaused '%s'\n", jobName)))
						}).To(Change(func() int {
							return len(adminAtcServer.ReceivedRequests())
						}).By(2))
					})
				})
			})

			Context("when user does NOT own the pipeline's team or pipeline/job doesn't exist", func() {
				BeforeEach(func() {
					randomApiPath := fmt.Sprintf("/api/v1/teams/random-team/pipelines/random-pipeline/jobs/random-job/unpause")
					adminAtcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("PUT", randomApiPath),
							ghttp.RespondWith(http.StatusNotFound, nil),
						),
					)
				})
				It("exits 1 and outputs the corresponding error", func() {
					Expect(func() {
						flyCmd = exec.Command(flyPath, "-t", "some-target", "unpause-job", "-j", "random-pipeline/random-job", "--team", "random-team")
						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())
						Eventually(sess.Err).Should(gbytes.Say(`random-pipeline/random-job not found on team random-team`))
						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(1))
					}).To(Change(func() int {
						return len(adminAtcServer.ReceivedRequests())
					}).By(2))
				})
			})

			Context("when a job is failed to be unpaused using the API", func() {
				BeforeEach(func() {
					apiPath := fmt.Sprintf("/api/v1/teams/main/pipelines/%s/jobs/%s/unpause", pipelineName, jobName)
					adminAtcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("PUT", apiPath),
							ghttp.RespondWith(http.StatusInternalServerError, nil),
						),
					)
				})

				It("exits 1 and outputs an error", func() {
					Expect(func() {
						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess.Err).Should(gbytes.Say(`error`))

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(1))
					}).To(Change(func() int {
						return len(adminAtcServer.ReceivedRequests())
					}).By(2))
				})
			})
		})

		Context("when the job flag is not provided", func() {
			BeforeEach(func() {
				flyCmd = exec.Command(flyPath, "-t", "some-target", "unpause-job")
			})

			It("exits 1 and outputs an error", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess.Err).Should(gbytes.Say(`error`))

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))
			})
		})
	})
})
