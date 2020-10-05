package integration_test

import (
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
	Describe("resources", func() {
		var (
			flyCmd *exec.Cmd
		)

		Context("when pipeline name is not specified", func() {
			It("fails and says pipeline name is required", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "resources")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))

				Expect(sess.Err).To(gbytes.Say("error: the required flag `" + osFlag("p", "pipeline") + "' was not specified"))
			})
		})

		Context("when resources are returned from the API", func() {
			BeforeEach(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "resources", "--pipeline", "pipeline/branch:master")
				pipelineRef := atc.PipelineRef{Name: "pipeline", InstanceVars: atc.InstanceVars{"branch": "master"}}
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines/pipeline/resources", "instance_vars=%7B%22branch%22%3A%22master%22%7D"),
						ghttp.RespondWithJSONEncoded(200, []atc.Resource{
							{
								Name:                 "resource-1",
								PipelineID:           1,
								PipelineName:         pipelineRef.Name,
								PipelineInstanceVars: pipelineRef.InstanceVars,
								TeamName:             teamName,
								Type:                 "time",
							},
							{
								Name:                 "resource-2",
								PipelineID:           1,
								PipelineName:         pipelineRef.Name,
								PipelineInstanceVars: pipelineRef.InstanceVars,
								TeamName:             teamName,
								Type:                 "custom",
								PinnedVersion:        atc.Version{"some": "version"},
							},
							{
								Name:                 "resource-3",
								PipelineID:           1,
								PipelineName:         pipelineRef.Name,
								PipelineInstanceVars: pipelineRef.InstanceVars,
								TeamName:             teamName,
								Type:                 "mock",
								FailingToCheck:       true,
								CheckError:           "some check error",
							},
							{
								Name:                 "resource-4",
								PipelineID:           1,
								PipelineName:         pipelineRef.Name,
								PipelineInstanceVars: pipelineRef.InstanceVars,
								TeamName:             teamName,
								Type:                 "mock",
								FailingToCheck:       true,
								CheckSetupError:      "some check setup error",
							},
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
                "name": "resource-1",
                "pipeline_id": 1,
                "pipeline_name": "pipeline",
                "pipeline_instance_vars": {
                  "branch": "master"
                },
                "team_name": "main",
                "type": "time"
              },
              {
                "name": "resource-2",
                "pipeline_id": 1,
                "pipeline_name": "pipeline",
                "pipeline_instance_vars": {
                  "branch": "master"
                },
                "team_name": "main",
                "type": "custom",
                "pinned_version": {"some": "version"}
              },
              {
                "name": "resource-3",
                "pipeline_id": 1,
                "pipeline_name": "pipeline",
                "pipeline_instance_vars": {
                  "branch": "master"
                },
                "team_name": "main",
                "type": "mock",
								"failing_to_check": true,
								"check_error": "some check error"
              },
              {
                "name": "resource-4",
                "pipeline_id": 1,
                "pipeline_name": "pipeline",
                "pipeline_instance_vars": {
                  "branch": "master"
                },
                "team_name": "main",
                "type": "mock",
								"failing_to_check": true,
								"check_setup_error": "some check setup error"
              }
            ]`))
				})
			})

			It("shows the pipeline's resources", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))

				Expect(sess.Out).To(PrintTable(ui.Table{
					Data: []ui.TableRow{
						{{Contents: "resource-1"}, {Contents: "time"}, {Contents: "n/a"}, {Contents: "ok", Color: color.New(color.FgGreen)}},
						{{Contents: "resource-2"}, {Contents: "custom"}, {Contents: "some:version", Color: color.New(color.FgCyan)}, {Contents: "ok", Color: color.New(color.FgGreen)}},
						{{Contents: "resource-3"}, {Contents: "mock"}, {Contents: "n/a"}, {Contents: "some check error", Color: color.New(color.FgRed)}},
						{{Contents: "resource-4"}, {Contents: "mock"}, {Contents: "n/a"}, {Contents: "some check setup error", Color: color.New(color.FgRed, color.Bold)}},
					},
				}))
			})
		})

		Context("when the api returns an internal server error", func() {
			BeforeEach(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "resources", "-p", "pipeline")
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines/pipeline/resources"),
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
