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
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines/pipeline/resources", "vars.branch=%22master%22"),
						ghttp.RespondWithJSONEncoded(200, []atc.Resource{
							{
								Name:                 "resource-1",
								PipelineID:           1,
								PipelineName:         pipelineRef.Name,
								PipelineInstanceVars: pipelineRef.InstanceVars,
								TeamName:             teamName,
								Type:                 "time",
								Build: &atc.BuildSummary{
									ID:                   122,
									Name:                 "122",
									Status:               atc.StatusSucceeded,
									TeamName:             teamName,
									PipelineID:           1,
									PipelineName:         pipelineRef.Name,
									PipelineInstanceVars: pipelineRef.InstanceVars,
								},
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
								Build: &atc.BuildSummary{
									ID:                   123,
									Name:                 "123",
									Status:               atc.StatusFailed,
									TeamName:             teamName,
									PipelineID:           1,
									PipelineName:         pipelineRef.Name,
									PipelineInstanceVars: pipelineRef.InstanceVars,
								},
							},
							{
								Name:                 "resource-4",
								PipelineID:           1,
								PipelineName:         pipelineRef.Name,
								PipelineInstanceVars: pipelineRef.InstanceVars,
								TeamName:             teamName,
								Type:                 "mock",
								Build: &atc.BuildSummary{
									ID:                   124,
									Name:                 "124",
									Status:               atc.StatusErrored,
									TeamName:             teamName,
									PipelineID:           1,
									PipelineName:         pipelineRef.Name,
									PipelineInstanceVars: pipelineRef.InstanceVars,
								},
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
                "type": "time",
								"build": {
									"id": 122,
									"name": "122",
									"pipeline_id": 1,
									"pipeline_name": "pipeline",
									"pipeline_instance_vars": {
										"branch": "master"
									},
									"team_name": "main",
									"status": "succeeded"
								}
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
								"build": {
									"id": 123,
									"name": "123",
									"pipeline_id": 1,
									"pipeline_name": "pipeline",
									"pipeline_instance_vars": {
										"branch": "master"
									},
									"team_name": "main",
									"status": "failed"
								}
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
								"build": {
									"id": 124,
									"name": "124",
									"pipeline_id": 1,
									"pipeline_name": "pipeline",
									"pipeline_instance_vars": {
										"branch": "master"
									},
									"team_name": "main",
									"status": "errored"
								}
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
						{{Contents: "resource-1"}, {Contents: "time"}, {Contents: "n/a"}, {Contents: "succeeded", Color: color.New(color.FgGreen)}},
						{{Contents: "resource-2"}, {Contents: "custom"}, {Contents: "some:version", Color: color.New(color.FgCyan)}, {Contents: "n/a", Color: color.New(color.Faint)}},
						{{Contents: "resource-3"}, {Contents: "mock"}, {Contents: "n/a"}, {Contents: "failed", Color: color.New(color.FgRed)}},
						{{Contents: "resource-4"}, {Contents: "mock"}, {Contents: "n/a"}, {Contents: "errored", Color: color.New(color.FgRed, color.Bold)}},
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

		Context("user is NOT targeting the same team the resource belongs to", func() {
			team := "diff-team"
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", fmt.Sprintf("/api/v1/teams/%s", team)),
						ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Team{
							Name: team,
						}),
					),
				)
			})

			Context("when pipeline name is not specified", func() {
				It("fails and says pipeline name is required", func() {
					flyCmd := exec.Command(flyPath, "-t", targetName, "resources", "--team", team)

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(1))

					Expect(sess.Err).To(gbytes.Say("error: the required flag `" + osFlag("p", "pipeline") + "' was not specified"))
				})
			})

			Context("when resources are returned from the API", func() {
				BeforeEach(func() {
					flyCmd = exec.Command(flyPath, "-t", targetName, "resources", "--pipeline", "pipeline/branch:master", "--team", team)
					pipelineRef := atc.PipelineRef{Name: "pipeline", InstanceVars: atc.InstanceVars{"branch": "master"}}
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/api/v1/teams/diff-team/pipelines/pipeline/resources", "vars.branch=%22master%22"),
							ghttp.RespondWithJSONEncoded(200, []atc.Resource{
								{
									Name:                 "resource-1",
									PipelineID:           1,
									PipelineName:         pipelineRef.Name,
									PipelineInstanceVars: pipelineRef.InstanceVars,
									TeamName:             team,
									Type:                 "time",
									Build: &atc.BuildSummary{
										ID:                   122,
										Name:                 "122",
										Status:               atc.StatusSucceeded,
										TeamName:             team,
										PipelineID:           1,
										PipelineName:         pipelineRef.Name,
										PipelineInstanceVars: pipelineRef.InstanceVars,
									},
								},
								{
									Name:                 "resource-2",
									PipelineID:           1,
									PipelineName:         pipelineRef.Name,
									PipelineInstanceVars: pipelineRef.InstanceVars,
									TeamName:             team,
									Type:                 "custom",
									PinnedVersion:        atc.Version{"some": "version"},
								},
								{
									Name:                 "resource-3",
									PipelineID:           1,
									PipelineName:         pipelineRef.Name,
									PipelineInstanceVars: pipelineRef.InstanceVars,
									TeamName:             team,
									Type:                 "mock",
									Build: &atc.BuildSummary{
										ID:                   123,
										Name:                 "123",
										Status:               atc.StatusFailed,
										TeamName:             team,
										PipelineID:           1,
										PipelineName:         pipelineRef.Name,
										PipelineInstanceVars: pipelineRef.InstanceVars,
									},
								},
								{
									Name:                 "resource-4",
									PipelineID:           1,
									PipelineName:         pipelineRef.Name,
									PipelineInstanceVars: pipelineRef.InstanceVars,
									TeamName:             team,
									Type:                 "mock",
									Build: &atc.BuildSummary{
										ID:                   124,
										Name:                 "124",
										Status:               atc.StatusErrored,
										TeamName:             team,
										PipelineID:           1,
										PipelineName:         pipelineRef.Name,
										PipelineInstanceVars: pipelineRef.InstanceVars,
									},
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
                "team_name": "diff-team",
                "type": "time",
								"build": {
									"id": 122,
									"name": "122",
									"pipeline_id": 1,
									"pipeline_name": "pipeline",
									"pipeline_instance_vars": {
										"branch": "master"
									},
									"team_name": "diff-team",
									"status": "succeeded"
								}
              },
              {
                "name": "resource-2",
                "pipeline_id": 1,
                "pipeline_name": "pipeline",
                "pipeline_instance_vars": {
                  "branch": "master"
                },
                "team_name": "diff-team",
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
                "team_name": "diff-team",
                "type": "mock",
								"build": {
									"id": 123,
									"name": "123",
									"pipeline_id": 1,
									"pipeline_name": "pipeline",
									"pipeline_instance_vars": {
										"branch": "master"
									},
									"team_name": "diff-team",
									"status": "failed"
								}
              },
              {
                "name": "resource-4",
                "pipeline_id": 1,
                "pipeline_name": "pipeline",
                "pipeline_instance_vars": {
                  "branch": "master"
                },
                "team_name": "diff-team",
                "type": "mock",
								"build": {
									"id": 124,
									"name": "124",
									"pipeline_id": 1,
									"pipeline_name": "pipeline",
									"pipeline_instance_vars": {
										"branch": "master"
									},
									"team_name": "diff-team",
									"status": "errored"
								}
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
							{{Contents: "resource-1"}, {Contents: "time"}, {Contents: "n/a"}, {Contents: "succeeded", Color: color.New(color.FgGreen)}},
							{{Contents: "resource-2"}, {Contents: "custom"}, {Contents: "some:version", Color: color.New(color.FgCyan)}, {Contents: "n/a", Color: color.New(color.Faint)}},
							{{Contents: "resource-3"}, {Contents: "mock"}, {Contents: "n/a"}, {Contents: "failed", Color: color.New(color.FgRed)}},
							{{Contents: "resource-4"}, {Contents: "mock"}, {Contents: "n/a"}, {Contents: "errored", Color: color.New(color.FgRed, color.Bold)}},
						},
					}))
				})
			})

			Context("when the api returns an internal server error", func() {
				BeforeEach(func() {
					flyCmd = exec.Command(flyPath, "-t", targetName, "resources", "-p", "pipeline", "--team", team)
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/api/v1/teams/diff-team/pipelines/pipeline/resources"),
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
})
