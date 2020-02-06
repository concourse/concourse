package integration_test

import (
	"fmt"
	"os/exec"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/ui"
	"github.com/fatih/color"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Fly CLI", func() {
	Describe("pipeline-overview", func() {
		var (
			flyCmd     *exec.Cmd
			sampleJobs []atc.Job
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

		Context("when resources are returned from the API", func() {
			createResource := func(num int, pinnedVersion atc.Version, resourceType string) atc.Resource {
				return atc.Resource{
					Name:          fmt.Sprintf("resource-%d", num),
					PinnedVersion: pinnedVersion,
					Type:          resourceType,
				}
			}

			BeforeEach(func() {
				pipelineName := "pipeline"
				flyCmd = exec.Command(flyPath, "-t", targetName, "resources", "--pipeline", pipelineName)
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines/pipeline/resources"),
						ghttp.RespondWithJSONEncoded(200, []atc.Resource{
							createResource(1, nil, "time"),
							createResource(2, atc.Version{"some": "version"}, "custom"),
						}),
					),
				)
			})

			It("shows the pipeline's resources", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))

				Expect(sess.Out).To(PrintTable(ui.Table{
					Data: []ui.TableRow{
						{{Contents: "resource-1"}, {Contents: "time"}, {Contents: "n/a"}},
						{{Contents: "resource-2"}, {Contents: "custom"}, {Contents: "some:version", Color: color.New(color.FgCyan)}},
					},
				}))
			})
		})

		pipelineName := "pipeline"
		Context("when jobs are returned from the API", func() {
			createJob := func(num int, paused bool, status string, nextStatus string) atc.Job {
				var (
					build     *atc.Build
					nextBuild *atc.Build
				)
				if status != "" {
					build = &atc.Build{Status: status}
				}
				if nextStatus != "" {
					nextBuild = &atc.Build{Status: nextStatus}
				}

				return atc.Job{
					Name:          fmt.Sprintf("job-%d", num),
					Paused:        paused,
					FinishedBuild: build,
					NextBuild:     nextBuild,
				}
			}

			sampleJobs = []atc.Job{
				createJob(1, false, "succeeded", "started"),
				createJob(2, true, "failed", ""),
				createJob(3, false, "", ""),
			}

			BeforeEach(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "jobs", "--pipeline", pipelineName)
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines/pipeline/jobs"),
						ghttp.RespondWithJSONEncoded(200, sampleJobs),
					),
				)
			})

			It("shows the pipeline's jobs", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))

				Expect(sess.Out).To(PrintTable(ui.Table{
					Data: []ui.TableRow{
						{{Contents: "job-1"}, {Contents: "no"}, {Contents: "succeeded"}, {Contents: "started"}},
						{{Contents: "job-2"}, {Contents: "yes", Color: color.New(color.FgCyan)}, {Contents: "failed"}, {Contents: "n/a"}},
						{{Contents: "job-3"}, {Contents: "no"}, {Contents: "n/a"}, {Contents: "n/a"}},
					},
				}))
			})
		})

		Context("when the api returns an internal server error", func() {
			BeforeEach(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "pipeline-overview", "-p", "pipeline")
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines/pipeline/resources"),
						ghttp.RespondWith(500, ""),
					),
				)
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
