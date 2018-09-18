package integration_test

import (
	"fmt"
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
			BeforeEach(func() {
				pipelineName := "pipeline"
				flyCmd = exec.Command(flyPath, "-t", targetName, "jobs", "--pipeline", pipelineName)
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines/pipeline/jobs"),
						ghttp.RespondWithJSONEncoded(200, []atc.Job{
							createJob(1, false, "succeeded", "started"),
							createJob(2, true, "failed", ""),
							createJob(3, false, "", ""),
						}),
					),
				)
			})

			Context("when --json is given", func() {
				BeforeEach(func() {
					flyCmd.Args = append(flyCmd.Args, "--json")
				})

				It("prints response in json as stdout", func() {
					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess).Should(gexec.Exit(0))
					Expect(sess.Out.Contents()).To(MatchJSON(`[
              {
                "id": 0,
                "name": "job-1",
                "pipeline_name": "",
                "team_name": "",
                "next_build": {
                  "id": 0,
                  "team_name": "",
                  "name": "",
                  "status": "started",
                  "api_url": ""
                },
                "finished_build": {
                  "id": 0,
                  "team_name": "",
                  "name": "",
                  "status": "succeeded",
                  "api_url": ""
                },
                "inputs": null,
                "outputs": null,
                "groups": null
              },
              {
                "id": 0,
                "name": "job-2",
                "pipeline_name": "",
                "team_name": "",
                "paused": true,
                "next_build": null,
                "finished_build": {
                  "id": 0,
                  "team_name": "",
                  "name": "",
                  "status": "failed",
                  "api_url": ""
                },
                "inputs": null,
                "outputs": null,
                "groups": null
              },
              {
                "id": 0,
                "name": "job-3",
                "pipeline_name": "",
                "team_name": "",
                "next_build": null,
                "finished_build": null,
                "inputs": null,
                "outputs": null,
                "groups": null
              }
            ]`))
				})
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
