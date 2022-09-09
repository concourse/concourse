package integration_test

import (
	"fmt"
	"net/http"
	"os/exec"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/ui"
	"github.com/fatih/color"
	. "github.com/onsi/ginkgo/v2"
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

		expectedURL := "/api/v1/teams/main/pipelines/pipeline/jobs"
		sampleJobJsonString := `[
              {
                "id": 1,
                "name": "job-1",
                "pipeline_id": 1,
                "pipeline_name": "pipeline",
                "pipeline_instance_vars": {
                  "branch": "master"
                },
                "team_name": "main",
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
                }
              },
              {
                "id": 2,
                "name": "job-2",
                "pipeline_id": 1,
                "pipeline_name": "pipeline",
                "pipeline_instance_vars": {
                  "branch": "master"
                },
                "team_name": "main",
                "paused": true,
                "next_build": null,
                "finished_build": {
                  "id": 0,
                  "team_name": "",
                  "name": "",
                  "status": "failed",
                  "api_url": ""
                }
              },
              {
                "id": 3,
                "name": "job-3",
                "pipeline_id": 1,
                "pipeline_name": "pipeline",
                "pipeline_instance_vars": {
                  "branch": "master"
                },
                "team_name": "main",
                "next_build": null,
                "finished_build": null
              }
            ]`
		var sampleJobs []atc.Job

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
			createJob := func(num int, pipelineRef atc.PipelineRef, paused bool, status, nextStatus atc.BuildStatus) atc.Job {
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
					ID:                   num,
					Name:                 fmt.Sprintf("job-%d", num),
					PipelineID:           1,
					PipelineName:         pipelineRef.Name,
					PipelineInstanceVars: pipelineRef.InstanceVars,
					TeamName:             teamName,
					Paused:               paused,
					FinishedBuild:        build,
					NextBuild:            nextBuild,
				}
			}

			pipelineRef := atc.PipelineRef{
				Name:         "pipeline",
				InstanceVars: atc.InstanceVars{"branch": "master"},
			}

			sampleJobs = []atc.Job{
				createJob(1, pipelineRef, false, atc.StatusSucceeded, atc.StatusStarted),
				createJob(2, pipelineRef, true, atc.StatusFailed, ""),
				createJob(3, pipelineRef, false, "", ""),
			}

			BeforeEach(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "jobs", "--pipeline", "pipeline/branch:master")
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL, "vars.branch=%22master%22"),
						ghttp.RespondWithJSONEncoded(200, sampleJobs),
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
					Expect(sess.Out.Contents()).To(MatchJSON(sampleJobJsonString))
				})
			})

			It("shows the pipeline's jobs", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))

				Expect(sess.Out).To(PrintTable(ui.Table{
					Data: []ui.TableRow{
						{{Contents: "job-1"}, {Contents: "no"}, {Contents: "succeeded"}, {Contents: "started", Color: color.New(color.FgGreen)}},
						{{Contents: "job-2"}, {Contents: "yes", Color: color.New(color.FgCyan)}, {Contents: "failed", Color: color.New(color.FgRed)}, {Contents: "n/a"}},
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
						ghttp.VerifyRequest("GET", expectedURL),
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

		Context("jobs for 'other-team'", func() {
			Context("using --team parameter", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/api/v1/teams/other-team"),
							ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Team{Name: "other-team"}),
						),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/api/v1/teams/other-team/pipelines/pipeline/jobs"),
							ghttp.RespondWithJSONEncoded(200, sampleJobs),
						),
					)
				})
				It("can list jobs in 'other-team'", func() {
					flyJobCmd := exec.Command(flyPath, "-t", targetName, "jobs", "-p", "pipeline", "--team", "other-team", "--json")
					sess, err := gexec.Start(flyJobCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess).Should(gexec.Exit(0))
					Expect(sess.Out.Contents()).To(MatchJSON(sampleJobJsonString))
				})
			})
		})
	})
})
