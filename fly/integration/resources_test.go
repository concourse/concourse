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
			createResource := func(num int, paused bool, resourceType string) atc.Resource {
				return atc.Resource{
					Name:   fmt.Sprintf("resource-%d", num),
					Paused: paused,
					Type:   resourceType,
				}
			}
			BeforeEach(func() {
				pipelineName := "pipeline"
				flyCmd = exec.Command(flyPath, "-t", targetName, "resources", "--pipeline", pipelineName)
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines/pipeline/resources"),
						ghttp.RespondWithJSONEncoded(200, []atc.Resource{
							createResource(1, false, "time"),
							createResource(2, true, "custom"),
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
                "pipeline_name": "",
                "team_name": "",
                "type": "time"
              },
              {
                "name": "resource-2",
                "pipeline_name": "",
                "team_name": "",
                "type": "custom",
				"paused": true
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
						{{Contents: "resource-1"}, {Contents: "no"}, {Contents: "time"}},
						{{Contents: "resource-2"}, {Contents: "yes", Color: color.New(color.FgCyan)}, {Contents: "custom"}},
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
