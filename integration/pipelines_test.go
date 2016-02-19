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
	Describe("pipelines", func() {
		var (
			flyCmd *exec.Cmd
		)

		BeforeEach(func() {
			flyCmd = exec.Command(flyPath, "-t", targetName, "pipelines")
		})

		Context("when pipelines are returned from the API", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/pipelines"),
						ghttp.RespondWithJSONEncoded(200, []atc.Pipeline{
							{Name: "pipeline-1-longer", URL: "/pipelines/pipeline-1", Paused: false},
							{Name: "pipeline-2", URL: "/pipelines/pipeline-2", Paused: true},
							{Name: "pipeline-3", URL: "/pipelines/pipeline-3", Paused: false},
						}),
					),
				)
			})

			It("lists them to the user", func() {
				Expect(flyCmd).To(PrintTable(ui.Table{
					Headers: ui.TableRow{
						{Contents: "name", Color: color.New(color.Bold)},
						{Contents: "paused", Color: color.New(color.Bold)},
					},
					Data: []ui.TableRow{
						{{Contents: "pipeline-1-longer"}, {Contents: "no"}},
						{{Contents: "pipeline-2"}, {Contents: "yes", Color: color.New(color.FgCyan)}},
						{{Contents: "pipeline-3"}, {Contents: "no"}},
					},
				}))

				Expect(flyCmd).To(HaveExited(0))
			})
		})

		Context("and the api returns an internal server error", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/pipelines"),
						ghttp.RespondWith(500, ""),
					),
				)
			})

			It("writes an error message to stderr", func() {
				sess, err := gexec.Start(flyCmd, nil, nil)
				Expect(err).ToNot(HaveOccurred())
				Eventually(sess.Err).Should(gbytes.Say("Unexpected Response"))
				Eventually(sess).Should(gexec.Exit(1))
			})
		})
	})
})
