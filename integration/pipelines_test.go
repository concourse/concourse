package integration_test

import (
	"os"
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

		Context("when pipelines are returned from the API", func() {
			Context("when no --all flag is given", func() {
				BeforeEach(func() {
					flyCmd = exec.Command(flyPath, "-t", targetName, "pipelines")
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines"),
							ghttp.RespondWithJSONEncoded(200, []atc.Pipeline{
								{Name: "pipeline-1-longer", Paused: false, Public: false},
								{Name: "pipeline-2", Paused: true, Public: false},
								{Name: "pipeline-3", Paused: false, Public: true},
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
                  "name": "pipeline-1-longer",
                  "paused": false,
                  "public": false,
                  "team_name": ""
                },
                {
                  "id": 0,
                  "name": "pipeline-2",
                  "paused": true,
                  "public": false,
                  "team_name": ""
                },
                {
                  "id": 0,
                  "name": "pipeline-3",
                  "paused": false,
                  "public": true,
                  "team_name": ""
                }
              ]`))
					})
				})

				It("only shows the team's pipelines", func() {
					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Eventually(sess).Should(gexec.Exit(0))

					Expect(sess.Out).To(PrintTable(ui.Table{
						Headers: ui.TableRow{
							{Contents: "name", Color: color.New(color.Bold)},
							{Contents: "paused", Color: color.New(color.Bold)},
							{Contents: "public", Color: color.New(color.Bold)},
						},
						Data: []ui.TableRow{
							{{Contents: "pipeline-1-longer"}, {Contents: "no"}, {Contents: "no"}},
							{{Contents: "pipeline-2"}, {Contents: "yes", Color: color.New(color.FgCyan)}, {Contents: "no"}},
							{{Contents: "pipeline-3"}, {Contents: "no"}, {Contents: "yes", Color: color.New(color.FgCyan)}},
						},
					}))
				})
			})

			Context("when --all is specified", func() {
				BeforeEach(func() {
					flyCmd = exec.Command(flyPath, "-t", targetName, "pipelines", "--all")
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/api/v1/pipelines"),
							ghttp.RespondWithJSONEncoded(200, []atc.Pipeline{
								{Name: "pipeline-1-longer", Paused: false, Public: false, TeamName: "main"},
								{Name: "pipeline-2", Paused: true, Public: false, TeamName: "main"},
								{Name: "pipeline-3", Paused: false, Public: true, TeamName: "main"},
								{Name: "foreign-pipeline-1", Paused: false, Public: true, TeamName: "other"},
								{Name: "foreign-pipeline-2", Paused: false, Public: true, TeamName: "other"},
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
                  "name": "pipeline-1-longer",
                  "paused": false,
                  "public": false,
                  "team_name": "main"
                },
                {
                  "id": 0,
                  "name": "pipeline-2",
                  "paused": true,
                  "public": false,
                  "team_name": "main"
                },
                {
                  "id": 0,
                  "name": "pipeline-3",
                  "paused": false,
                  "public": true,
                  "team_name": "main"
                },
                {
                  "id": 0,
                  "name": "foreign-pipeline-1",
                  "paused": false,
                  "public": true,
                  "team_name": "other"
                },
                {
                  "id": 0,
                  "name": "foreign-pipeline-2",
                  "paused": false,
                  "public": true,
                  "team_name": "other"
                }
              ]`))
					})
				})

				It("includes team and shared pipelines, with a team name column", func() {
					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Eventually(sess).Should(gexec.Exit(0))

					Expect(sess.Out).To(PrintTable(ui.Table{
						Headers: ui.TableRow{
							{Contents: "name", Color: color.New(color.Bold)},
							{Contents: "team", Color: color.New(color.Bold)},
							{Contents: "paused", Color: color.New(color.Bold)},
							{Contents: "public", Color: color.New(color.Bold)},
						},
						Data: []ui.TableRow{
							{{Contents: "pipeline-1-longer"}, {Contents: "main"}, {Contents: "no"}, {Contents: "no"}},
							{{Contents: "pipeline-2"}, {Contents: "main"}, {Contents: "yes", Color: color.New(color.FgCyan)}, {Contents: "no"}},
							{{Contents: "pipeline-3"}, {Contents: "main"}, {Contents: "no"}, {Contents: "yes", Color: color.New(color.FgCyan)}},
							{{Contents: "foreign-pipeline-1"}, {Contents: "other"}, {Contents: "no"}, {Contents: "yes", Color: color.New(color.FgCyan)}},
							{{Contents: "foreign-pipeline-2"}, {Contents: "other"}, {Contents: "no"}, {Contents: "yes", Color: color.New(color.FgCyan)}},
						},
					}))
				})
			})

			Context("completion", func() {
				BeforeEach(func() {
					flyCmd = exec.Command(flyPath, "-t", targetName, "get-pipeline", "-p", "some-")
					flyCmd.Env = append(os.Environ(), "GO_FLAGS_COMPLETION=1")
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines"),
							ghttp.RespondWithJSONEncoded(200, []atc.Pipeline{
								{Name: "some-pipeline-1", Paused: false, Public: false},
								{Name: "some-pipeline-2", Paused: false, Public: false},
								{Name: "another-pipeline", Paused: false, Public: false},
							}),
						),
					)
				})

				It("returns all matching pipelines", func() {
					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Eventually(sess).Should(gexec.Exit(0))
					Eventually(sess.Out).Should(gbytes.Say("some-pipeline-1"))
					Eventually(sess.Out).Should(gbytes.Say("some-pipeline-2"))
					Eventually(sess.Out).ShouldNot(gbytes.Say("another-pipeline"))
				})
			})
		})

		Context("and the api returns an internal server error", func() {
			BeforeEach(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "pipelines")
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines"),
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
