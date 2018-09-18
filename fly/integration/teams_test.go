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
								Auth: map[string][]string{
									"groups": []string{},
									"users":  []string{},
								},
							},
							{
								ID:   2,
								Name: "a-team",
								Auth: map[string][]string{
									"groups": []string{"github:github-org"},
									"users":  []string{},
								},
							},
							{
								ID:   3,
								Name: "b-team",
								Auth: map[string][]string{
									"groups": []string{},
									"users":  []string{"github:github-user"},
								},
							},
							{
								ID:   4,
								Name: "c-team",
								Auth: map[string][]string{
									"users":  []string{"github:github-user"},
									"groups": []string{"github:github-org"},
								},
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
						{{Contents: "c-team"}},
						{{Contents: "main"}},
					},
				}))
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
                "id": 1,
                "name": "main",
				"auth": {"groups":[], "users":[]}
              },
              {
                "id": 2,
                "name": "a-team",
				"auth": {
					"groups": ["github:github-org"],
					"users": []
				}
              },
              {
                "id": 3,
                "name": "b-team",
				"auth": {
					"users": ["github:github-user"],
					"groups": []
				}
              },
              {
				"id": 4,
				"name": "c-team",
				"auth": {
					"groups":["github:github-org"],
					"users":["github:github-user"]
				}
              }
            ]`))
				})
			})

			Context("when the details flag is specified", func() {
				BeforeEach(func() {
					flyCmd.Args = append(flyCmd.Args, "--details")
				})

				It("lists them to the user, ordered by name", func() {
					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess).Should(gexec.Exit(0))
					Expect(sess.Out).To(PrintTable(ui.Table{
						Headers: ui.TableRow{
							{Contents: "name", Color: color.New(color.Bold)},
							{Contents: "auth", Color: color.New(color.Bold)},
						},
						Data: []ui.TableRow{
							{{Contents: "a-team"}, {Contents: "none"}, {Contents: "github:github-org"}},
							{{Contents: "b-team"}, {Contents: "github:github-user"}, {Contents: "none"}},
							{{Contents: "c-team"}, {Contents: "github:github-user"}, {Contents: "github:github-org"}},
							{{Contents: "main"}, {Contents: "all"}, {Contents: "none"}},
						},
					}))
				})
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
