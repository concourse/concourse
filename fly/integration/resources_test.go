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
			createResource := func(num int, pipelineRef atc.PipelineRef, pinnedVersion atc.Version, resourceType string) atc.Resource {
				return atc.Resource{
					Name:                 fmt.Sprintf("resource-%d", num),
					PipelineID:           1,
					PipelineName:         pipelineRef.Name,
					PipelineInstanceVars: pipelineRef.InstanceVars,
					TeamName:             teamName,
					PinnedVersion:        pinnedVersion,
					Type:                 resourceType,
				}
			}

			BeforeEach(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "resources", "--pipeline", "pipeline/branch:master")
				pipelineRef := atc.PipelineRef{Name: "pipeline", InstanceVars: atc.InstanceVars{"branch": "master"}}
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines/pipeline/resources", "instance_vars=%7B%22branch%22%3A%22master%22%7D"),
						ghttp.RespondWithJSONEncoded(200, []atc.Resource{
							createResource(1, pipelineRef, nil, "time"),
							createResource(2, pipelineRef, atc.Version{"some": "version"}, "custom"),
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
						{{Contents: "resource-1"}, {Contents: "time"}, {Contents: "n/a"}},
						{{Contents: "resource-2"}, {Contents: "custom"}, {Contents: "some:version", Color: color.New(color.FgCyan)}},
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
