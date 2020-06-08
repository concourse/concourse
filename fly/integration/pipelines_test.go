package integration_test

import (
	"os"
	"os/exec"
	"time"

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
	Describe("pipelines", func() {
		var (
			flyCmd *exec.Cmd
		)

		JustBeforeEach(func() {
			flyCmd.Args = append([]string{flyCmd.Args[0], "--print-table-headers"}, flyCmd.Args[1:]...)
		})

		Context("when pipelines are returned from the API", func() {
			BeforeEach(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "pipelines")
			})
			Context("when no --all flag is given", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines"),
							ghttp.RespondWithJSONEncoded(200, []atc.Pipeline{
								{Name: "pipeline-1-longer", Paused: false, Public: false, LastUpdated: 1},
								{Name: "pipeline-2", Paused: true, Public: false, LastUpdated: 1},
								{Name: "pipeline-3", Paused: false, Public: true, LastUpdated: 1},
								{Name: "archived-pipeline", Paused: false, Archived: true, Public: true, LastUpdated: 1},
							}),
						),
					)
				})

				Context("when --json is given", func() {
					BeforeEach(func() {
						flyCmd.Args = append(flyCmd.Args, "--json")
					})

					It("prints non-archived pipelines in json to stdout", func() {
						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess).Should(gexec.Exit(0))
						Expect(sess.Out.Contents()).To(MatchJSON(`[
                {
                  "id": 0,
                  "name": "pipeline-1-longer",
                  "paused": false,
                  "public": false,
                  "archived": false,
                  "team_name": "",
                  "last_updated": 1
                },
                {
                  "id": 0,
                  "name": "pipeline-2",
                  "paused": true,
                  "public": false,
                  "archived": false,
                  "team_name": "",
                  "last_updated": 1
                },
                {
                  "id": 0,
                  "name": "pipeline-3",
                  "paused": false,
                  "public": true,
                  "archived": false,
                  "team_name": "",
                  "last_updated": 1
                }
              ]`))
					})
				})

				It("only shows the team's pipelines", func() {
					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Eventually(sess).Should(gexec.Exit(0))

					Expect(sess.Out).To(PrintTableWithHeaders(ui.Table{
						Headers: ui.TableRow{
							{Contents: "name", Color: color.New(color.Bold)},
							{Contents: "paused", Color: color.New(color.Bold)},
							{Contents: "public", Color: color.New(color.Bold)},
							{Contents: "last updated", Color: color.New(color.Bold)},
						},
						Data: []ui.TableRow{
							{{Contents: "pipeline-1-longer"}, {Contents: "no"}, {Contents: "no"}, {Contents: time.Unix(1, 0).String()}},
							{{Contents: "pipeline-2"}, {Contents: "yes", Color: color.New(color.FgCyan)}, {Contents: "no"}, {Contents: time.Unix(1, 0).String()}},
							{{Contents: "pipeline-3"}, {Contents: "no"}, {Contents: "yes", Color: color.New(color.FgCyan)}, {Contents: time.Unix(1, 0).String()}},
						},
					}))
				})

				It("does not print archived pipelines", func() {
					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Eventually(sess).Should(gexec.Exit(0))

					Expect(sess.Out).ToNot(gbytes.Say("archived-pipeline"))
				})
			})

			Context("when --all is specified", func() {
				BeforeEach(func() {
					flyCmd.Args = append(flyCmd.Args, "--all")
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/api/v1/pipelines"),
							ghttp.RespondWithJSONEncoded(200, []atc.Pipeline{
								{Name: "pipeline-1-longer", Paused: false, Public: false, TeamName: "main", LastUpdated: 1},
								{Name: "pipeline-2", Paused: true, Public: false, TeamName: "main", LastUpdated: 1},
								{Name: "pipeline-3", Paused: false, Public: true, TeamName: "main", LastUpdated: 1},
								{Name: "archived-pipeline", Paused: false, Archived: true, Public: true, TeamName: "main", LastUpdated: 1},
								{Name: "foreign-pipeline-1", Paused: false, Public: true, TeamName: "other", LastUpdated: 1},
								{Name: "foreign-pipeline-2", Paused: false, Public: true, TeamName: "other", LastUpdated: 1},
								{Name: "foreign-archived-pipeline", Paused: false, Archived: true, Public: true, TeamName: "other", LastUpdated: 1},
							}),
						),
					)
				})

				Context("when --json is given", func() {
					BeforeEach(func() {
						flyCmd.Args = append(flyCmd.Args, "--json")
					})

					It("prints non-archived pipelines in json to stdout", func() {
						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess).Should(gexec.Exit(0))
						Expect(sess.Out.Contents()).To(MatchJSON(`[
                {
                  "id": 0,
                  "name": "pipeline-1-longer",
                  "paused": false,
                  "public": false,
                  "archived": false,
                  "team_name": "main",
                  "last_updated": 1
                },
                {
                  "id": 0,
                  "name": "pipeline-2",
                  "paused": true,
                  "public": false,
                  "archived": false,
                  "team_name": "main",
                  "last_updated": 1
                },
                {
                  "id": 0,
                  "name": "pipeline-3",
                  "paused": false,
                  "public": true,
                  "archived": false,
                  "team_name": "main",
                  "last_updated": 1
                },
                {
                  "id": 0,
                  "name": "foreign-pipeline-1",
                  "paused": false,
                  "public": true,
                  "archived": false,
                  "team_name": "other",
                  "last_updated": 1
                },
                {
                  "id": 0,
                  "name": "foreign-pipeline-2",
                  "paused": false,
                  "public": true,
                  "archived": false,
                  "team_name": "other",
                  "last_updated": 1
                }
              ]`))
					})
				})

				It("outputs pipelines across all teams", func() {
					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Eventually(sess).Should(gexec.Exit(0))

					Expect(sess.Out).To(PrintTableWithHeaders(ui.Table{
						Headers: ui.TableRow{
							{Contents: "name", Color: color.New(color.Bold)},
							{Contents: "team", Color: color.New(color.Bold)},
							{Contents: "paused", Color: color.New(color.Bold)},
							{Contents: "public", Color: color.New(color.Bold)},
							{Contents: "last updated", Color: color.New(color.Bold)},
						},
						Data: []ui.TableRow{
							{{Contents: "pipeline-1-longer"}, {Contents: "main"}, {Contents: "no"}, {Contents: "no"}, {Contents: time.Unix(1, 0).String()}},
							{{Contents: "pipeline-2"}, {Contents: "main"}, {Contents: "yes", Color: color.New(color.FgCyan)}, {Contents: "no"}, {Contents: time.Unix(1, 0).String()}},
							{{Contents: "pipeline-3"}, {Contents: "main"}, {Contents: "no"}, {Contents: "yes", Color: color.New(color.FgCyan)}, {Contents: time.Unix(1, 0).String()}},
							{{Contents: "foreign-pipeline-1"}, {Contents: "other"}, {Contents: "no"}, {Contents: "yes", Color: color.New(color.FgCyan)}, {Contents: time.Unix(1, 0).String()}},
							{{Contents: "foreign-pipeline-2"}, {Contents: "other"}, {Contents: "no"}, {Contents: "yes", Color: color.New(color.FgCyan)}, {Contents: time.Unix(1, 0).String()}},
						},
					}))
				})

				It("does not print archived pipelines", func() {
					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Eventually(sess).Should(gexec.Exit(0))

					Expect(sess.Out).ToNot(gbytes.Say("archived-pipeline"))
				})
			})

			Context("when --include-archived is specified", func() {
				BeforeEach(func() {
					flyCmd.Args = append(flyCmd.Args, "--include-archived")
				})

				It("includes archived pipelines in the output", func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines"),
							ghttp.RespondWithJSONEncoded(200, []atc.Pipeline{
								{Name: "pipeline-1-longer", Paused: false, Public: false, TeamName: "main", LastUpdated: 1},
								{Name: "archived-pipeline", Paused: true, Archived: true, Public: true, TeamName: "main", LastUpdated: 1},
							}),
						),
					)

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Eventually(sess).Should(gexec.Exit(0))

					Expect(sess.Out).To(PrintTableWithHeaders(ui.Table{
						Headers: ui.TableRow{
							{Contents: "name", Color: color.New(color.Bold)},
							{Contents: "paused", Color: color.New(color.Bold)},
							{Contents: "public", Color: color.New(color.Bold)},
							{Contents: "archived", Color: color.New(color.Bold)},
							{Contents: "last updated", Color: color.New(color.Bold)},
						},
						Data: []ui.TableRow{
							{{Contents: "pipeline-1-longer"}, {Contents: "no"}, {Contents: "no"}, {Contents: "no"}, {Contents: time.Unix(1, 0).String()}},
							{{Contents: "archived-pipeline"}, {Contents: "yes"}, {Contents: "yes", Color: color.New(color.FgCyan)}, {Contents: "yes"}, {Contents: time.Unix(1, 0).String()}},
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
								{Name: "some-pipeline-1", Paused: false, Public: false, LastUpdated: 1},
								{Name: "some-pipeline-2", Paused: false, Public: false, LastUpdated: 1},
								{Name: "another-pipeline", Paused: false, Public: false, LastUpdated: 1},
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

		Context("when there are no pipelines", func() {
			BeforeEach(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "pipelines")
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines"),
						ghttp.RespondWithJSONEncoded(200, []atc.Pipeline{}),
					),
				)
			})

			Context("when --json is given", func() {
				BeforeEach(func() {
					flyCmd.Args = append(flyCmd.Args, "--json")
				})

				It("prints an empty list", func() {
					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Eventually(sess).Should(gexec.Exit(0))

					Expect(sess.Out.Contents()).To(MatchJSON(`[]`))
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
