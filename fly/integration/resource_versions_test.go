package integration_test

import (
	"fmt"
	"net/http"
	"os/exec"
	"strings"

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
	Describe("resource-versions", func() {
		var (
			flyCmd      *exec.Cmd
			queryParams = []string{"vars.branch=%22master%22", "limit=50"}
		)

		BeforeEach(func() {
			flyCmd = exec.Command(flyPath, "-t", targetName, "resource-versions", "-r", "pipeline/branch:master/foo")
		})

		Context("when pipelines are returned from the API", func() {
			Context("when no --all flag is given", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines/pipeline/resources/foo/versions", strings.Join(queryParams, "&")),
							ghttp.RespondWithJSONEncoded(200, []atc.ResourceVersion{
								{ID: 3, Version: atc.Version{"version": "3", "another": "field"}, Enabled: true},
								{ID: 2, Version: atc.Version{"version": "2", "another": "field"}, Enabled: false},
								{ID: 1, Version: atc.Version{"version": "1", "another": "field"}, Enabled: true},
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
                  "id": 3,
									"version": {"version":"3","another":"field"},
									"enabled": true
                },
                {
                  "id": 2,
									"version": {"version":"2","another":"field"},
									"enabled": false
                },
                {
                  "id": 1,
									"version": {"version":"1","another":"field"},
									"enabled": true
                }
              ]`))
					})
				})

				It("lists the resource versions", func() {
					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Eventually(sess).Should(gexec.Exit(0))

					Expect(sess.Out).To(PrintTable(ui.Table{
						Headers: ui.TableRow{
							{Contents: "id", Color: color.New(color.Bold)},
							{Contents: "version", Color: color.New(color.Bold)},
							{Contents: "enabled", Color: color.New(color.Bold)},
						},
						Data: []ui.TableRow{
							{{Contents: "3"}, {Contents: "another:field,version:3"}, {Contents: "yes"}},
							{{Contents: "2"}, {Contents: "another:field,version:2"}, {Contents: "no"}},
							{{Contents: "1"}, {Contents: "another:field,version:1"}, {Contents: "yes"}},
						},
					}))
				})
			})
		})

		Context("and the api returns an internal server error", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines/pipeline/resources/foo/versions", strings.Join(queryParams, "&")),
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

		Context("user is NOT targeting the same team the resource type belongs to", func() {
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

			BeforeEach(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "resource-versions", "-r", "pipeline/branch:master/foo", "--team", team)
			})

			Context("when pipelines are returned from the API", func() {
				Context("when no --all flag is given", func() {
					BeforeEach(func() {
						atcServer.AppendHandlers(
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("GET", "/api/v1/teams/diff-team/pipelines/pipeline/resources/foo/versions", strings.Join(queryParams, "&")),
								ghttp.RespondWithJSONEncoded(200, []atc.ResourceVersion{
									{ID: 3, Version: atc.Version{"version": "3", "another": "field"}, Enabled: true},
									{ID: 2, Version: atc.Version{"version": "2", "another": "field"}, Enabled: false},
									{ID: 1, Version: atc.Version{"version": "1", "another": "field"}, Enabled: true},
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
                  "id": 3,
									"version": {"version":"3","another":"field"},
									"enabled": true
                },
                {
                  "id": 2,
									"version": {"version":"2","another":"field"},
									"enabled": false
                },
                {
                  "id": 1,
									"version": {"version":"1","another":"field"},
									"enabled": true
                }
              ]`))
						})
					})

					It("lists the resource versions", func() {
						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())
						Eventually(sess).Should(gexec.Exit(0))

						Expect(sess.Out).To(PrintTable(ui.Table{
							Headers: ui.TableRow{
								{Contents: "id", Color: color.New(color.Bold)},
								{Contents: "version", Color: color.New(color.Bold)},
								{Contents: "enabled", Color: color.New(color.Bold)},
							},
							Data: []ui.TableRow{
								{{Contents: "3"}, {Contents: "another:field,version:3"}, {Contents: "yes"}},
								{{Contents: "2"}, {Contents: "another:field,version:2"}, {Contents: "no"}},
								{{Contents: "1"}, {Contents: "another:field,version:1"}, {Contents: "yes"}},
							},
						}))
					})
				})
			})

			Context("and the api returns an internal server error", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/api/v1/teams/diff-team/pipelines/pipeline/resources/foo/versions", strings.Join(queryParams, "&")),
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
