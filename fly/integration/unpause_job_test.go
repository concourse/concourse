package integration_test

import (
	"fmt"
	"net/http"
	"os/exec"

	"github.com/concourse/concourse/atc"
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
			queryParams  string
			pipelineRef  atc.PipelineRef
		)

		BeforeEach(func() {
			pipelineName = "pipeline"
			jobName = "job-name-potato"
			pipelineRef = atc.PipelineRef{Name: pipelineName, InstanceVars: atc.InstanceVars{"branch": "master"}}
			fullJobName = fmt.Sprintf("%s/%s", pipelineRef.String(), jobName)
			apiPath = fmt.Sprintf("/api/v1/teams/main/pipelines/%s/jobs/%s/unpause", pipelineName, jobName)
			queryParams = "instance_vars=%7B%22branch%22%3A%22master%22%7D"

			flyCmd = exec.Command(flyPath, "-t", targetName, "unpause-job", "-j", fullJobName)
		})

		Context("when the job flag is provided", func() {
			Context("when user and pipeline belong to the same team", func() {
				Context("user is targeting the same team that the pipeline belongs to", func() {
					BeforeEach(func() {
						atcServer.AppendHandlers(
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("PUT", apiPath, queryParams),
								ghttp.RespondWith(http.StatusOK, nil),
							),
						)
					})

					It("successfully unpauses the job", func() {
						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))
						Eventually(sess).Should(gbytes.Say(fmt.Sprintf("unpaused '%s'\n", jobName)))
					})
				})

				Context("user is NOT targeting the same team that the pipeline belongs to", func() {
					BeforeEach(func() {
						apiPath = fmt.Sprintf("/api/v1/teams/other-team/pipelines/%s/jobs/%s/unpause", pipelineName, jobName)

						atcServer.AppendHandlers(
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("GET", "/api/v1/teams/other-team"),
								ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Team{
									Name: "other-team",
								}),
							),
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("PUT", apiPath, queryParams),
								ghttp.RespondWith(http.StatusOK, nil),
							),
						)
					})

					It("successfully unpauses the job", func() {
						flyCmd = exec.Command(flyPath, "-t", targetName, "unpause-job", "-j", fullJobName, "--team", "other-team")
						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())
						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))
						Eventually(sess).Should(gbytes.Say(fmt.Sprintf("unpaused '%s'\n", jobName)))
					})
				})
			})

			Context("when unpause-job fails", func() {
				BeforeEach(func() {
					apiPath := fmt.Sprintf("/api/v1/teams/main/pipelines/%s/jobs/%s/unpause", pipelineName, jobName)
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("PUT", apiPath, queryParams),
							ghttp.RespondWith(http.StatusInternalServerError, nil),
						),
					)
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

		Context("when the job flag is not provided", func() {
			BeforeEach(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "unpause-job")
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
