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
			flyCmd *exec.Cmd
		)

		Context("when the job flag is provided", func() {
			pipelineName := "pipeline"
			jobName := "job-name-potato"
			fullJobName := fmt.Sprintf("%s/%s", pipelineName, jobName)

			BeforeEach(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "unpause-job", "-j", fullJobName)
			})

			Context("when a job is unpaused using the API", func() {
				BeforeEach(func() {
					apiPath := fmt.Sprintf("/api/v1/teams/main/pipelines/%s/jobs/%s/unpause", pipelineName, jobName)
					atcServer.AppendHandlers(
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
						return len(atcServer.ReceivedRequests())
					}).By(2))
				})
			})

			Context("when a job is unpaused using the API", func() {
				BeforeEach(func() {
					apiPath := fmt.Sprintf("/api/v1/teams/main/pipelines/%s/jobs/%s/unpause", pipelineName, jobName)
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("PUT", apiPath),
							ghttp.RespondWith(http.StatusInternalServerError, nil),
						),
					)
				})

				It("exists 1 and outputs an error", func() {
					Expect(func() {
						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess.Err).Should(gbytes.Say(`error`))

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(1))
					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(2))
				})
			})
		})

		Context("when the job flag is not provided", func() {
			BeforeEach(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "unpause-job")
			})

			It("exists 1 and outputs an error", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess.Err).Should(gbytes.Say(`error`))

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))
			})
		})
	})
})
