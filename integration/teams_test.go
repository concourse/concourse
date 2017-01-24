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
	Describe("teams", func() {
		var (
			flyCmd *exec.Cmd
		)

		BeforeEach(func() {
			flyCmd = exec.Command(flyPath, "-t", targetName, "teams")
		})

		Context("when teams are returned from the API", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams"),
						ghttp.RespondWithJSONEncoded(200, []atc.Team{
							{
								ID:   1,
								Name: "main",
							},
							{
								ID:   2,
								Name: "a-team",
							},
							{
								ID:   3,
								Name: "b-team",
							},
						}),
					),
				)
			})

			It("lists them to the user, ordered by name", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))
				Expect(sess.Out).To(PrintTable(ui.Table{
					Headers: ui.TableRow{
						{Contents: "name", Color: color.New(color.Bold)},
					},
					Data: []ui.TableRow{
						{{Contents: "a-team"}},
						{{Contents: "b-team"}},
						{{Contents: "main"}},
					},
				}))
			})
		})

		Context("and the api returns an internal server error", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams"),
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
