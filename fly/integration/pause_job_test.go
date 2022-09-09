package integration_test

import (
	"fmt"
	"net/http"
	"os/exec"

	"github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Fly CLI", func() {
	Describe("Pause Job", func() {
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
			apiPath = fmt.Sprintf("/api/v1/teams/main/pipelines/%s/jobs/%s/pause", pipelineName, jobName)
			queryParams = "vars.branch=%22master%22"
		})

		Context("when user is on the same team as the given pipeline/job's team", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", apiPath, queryParams),
						ghttp.RespondWith(http.StatusOK, nil),
					),
				)
			})

			It("successfully pauses the job", func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "pause-job", "-j", fullJobName)
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))
				Eventually(sess.Out.Contents).Should(ContainSubstring(fmt.Sprintf("paused '%s'\n", jobName)))
			})
		})

		Context("user is NOT on the same team as the given pipeline/job's team", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", apiPath, queryParams),
						ghttp.RespondWith(http.StatusForbidden, nil),
					),
				)
			})

			It("fails to pause the job", func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "pause-job", "-j", fullJobName)
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))
				Eventually(sess.Err.Contents).Should(ContainSubstring("error"))
			})
		})

		Context("user is NOT currently targeted to the team the given pipeline/job belongs to", func() {
			BeforeEach(func() {
				apiPath = fmt.Sprintf("/api/v1/teams/other-team/pipelines/%s/jobs/%s/pause", pipelineName, jobName)

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/other-team"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Team{Name: "other-team"}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", apiPath, queryParams),
						ghttp.RespondWith(http.StatusOK, nil),
					),
				)
			})

			It("successfully pauses the job", func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "pause-job", "-j", fullJobName, "--team", "other-team")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))
				Eventually(sess).Should(gbytes.Say(fmt.Sprintf("paused '%s'\n", jobName)))
			})
		})

		Context("the pipeline/job does not exist", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/api/v1/teams/main/pipelines/doesnotexist-pipeline/jobs/doesnotexist-job/pause"),
						ghttp.RespondWith(http.StatusNotFound, nil),
					),
				)
			})

			It("returns an error", func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "pause-job", "-j", "doesnotexist-pipeline/doesnotexist-job")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))
				Eventually(sess.Err.Contents).Should(ContainSubstring("doesnotexist-pipeline/doesnotexist-job not found on team main"))
			})
		})

		Context("when pause-job fails", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", apiPath, queryParams),
						ghttp.RespondWith(http.StatusInternalServerError, nil),
					),
				)
			})

			It("exits 1 and outputs an error", func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "pause-job", "-j", fullJobName)
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess.Err).Should(gbytes.Say(`error`))
				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))
			})
		})
	})
})
