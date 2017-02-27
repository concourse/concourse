package integration_test

import (
	"os/exec"

	"github.com/concourse/atc"
	"github.com/concourse/fly/ui"
	"github.com/fatih/color"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Fly CLI", func() {
	Describe("jobs", func() {
		var (
			flyCmd *exec.Cmd
		)

		Context("when not specifying a pipeline name", func() {
			It("fails and says you should give a pipeline name", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "jobs")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))

				Expect(sess.Err).To(gbytes.Say("error: the required flag `" + osFlag("p", "pipeline") + "' was not specified"))
			})
		})

		Context("when jobs are returned from the API", func() {
			BeforeEach(func() {
				pipelineName := "pipeline"
				flyCmd = exec.Command(flyPath, "-t", targetName, "jobs", "--pipeline", pipelineName)
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines/pipeline/jobs"),
						ghttp.RespondWithJSONEncoded(200, []atc.Job{
							{Name: "job-1", URL: "/teams/main/pipelines/pipeline/jobs/job-1", Paused: false},
							{Name: "job-2", URL: "/teams/main/pipelines/pipeline/jobs/job-2", Paused: true},
							{Name: "job-3", URL: "/teams/main/pipelines/pipeline/jobs/job-3", Paused: false},
						}),
					),
				)
			})

			It("shows the pipeline's jobs", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))

				Expect(sess.Out).To(PrintTable(ui.Table{
					Headers: ui.TableRow{
						{Contents: "name", Color: color.New(color.Bold)},
						{Contents: "paused", Color: color.New(color.Bold)},
					},
					Data: []ui.TableRow{
						{{Contents: "job-1"}, {Contents: "no"}},
						{{Contents: "job-2"}, {Contents: "yes", Color: color.New(color.FgCyan)}},
						{{Contents: "job-3"}, {Contents: "no"}},
					},
				}))
			})
		})

		Context("when the api returns an internal server error", func() {
			BeforeEach(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "jobs", "-p", "pipeline")
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines/pipeline/jobs"),
						ghttp.RespondWith(500, ""),
					),
				)
			})

			It("writes an error message to stderr", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(1))
				Eventually(sess.Err).Should(gbytes.Say("Unexpected Response"))
			})
		})
	})
})
