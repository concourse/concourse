package integration_test

import (
	"fmt"
	"io"
	"net/http"
	"os/exec"

	"github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Fly CLI", func() {
	var (
		flyCmd    *exec.Cmd
		cmdParams []string
	)

	JustBeforeEach(func() {
		params := append([]string{"-t", targetName, "set-team", "--team-name", "venture"}, cmdParams...)
		flyCmd = exec.Command(flyPath, params...)
	})

	yes := func(stdin io.Writer) {
		fmt.Fprintf(stdin, "y\n")
	}

	no := func(stdin io.Writer) {
		fmt.Fprintf(stdin, "n\n")
	}

	Context("using a config file", func() {
		Describe("config validation", func() {
			Describe("no auth", func() {
				Context("auth config is missing auth for role", func() {
					BeforeEach(func() {
						cmdParams = []string{"-c", "fixtures/team_config_no_auth_for_role.yml"}
					})

					It("returns an error", func() {
						sess, err := gexec.Start(flyCmd, nil, nil)
						Expect(err).ToNot(HaveOccurred())
						Eventually(sess.Err).Should(gbytes.Say("You have not provided a list of users and groups for one of the roles in your config yaml."))
						Eventually(sess).Should(gexec.Exit(1))
					})
				})
			})
		})

		Describe("Display", func() {
			Context("Setting no auth", func() {
				Context("allow-all-users configuration provided", func() {
					BeforeEach(func() {
						cmdParams = []string{"-c", "fixtures/team_config.yml"}
						confirmHandlers()
					})

					It("show a warning about creating unauthenticated team for a given role", func() {
						stdin, err := flyCmd.StdinPipe()
						Expect(err).NotTo(HaveOccurred())

						sess, err := gexec.Start(flyCmd, nil, nil)
						Expect(err).ToNot(HaveOccurred())

						Eventually(sess.Out).Should(gbytes.Say("Team Name: venture"))

						Eventually(sess.Out).Should(gbytes.Say("Users \\(member\\):"))
						Eventually(sess.Out).Should(gbytes.Say("- local:some-user"))
						Eventually(sess.Out).Should(gbytes.Say("Groups \\(member\\):"))
						Eventually(sess.Out).Should(gbytes.Say("- none"))

						Eventually(sess.Out).Should(gbytes.Say("Users \\(owner\\):"))
						Eventually(sess.Out).Should(gbytes.Say("- local:some-admin"))
						Eventually(sess.Out).Should(gbytes.Say("Groups \\(owner\\):"))
						Eventually(sess.Out).Should(gbytes.Say("- none"))

						Eventually(sess.Out).Should(gbytes.Say("Users \\(viewer\\):"))
						Eventually(sess.Out).Should(gbytes.Say("- none"))
						Eventually(sess.Out).Should(gbytes.Say("Groups \\(viewer\\):"))
						Eventually(sess.Out).Should(gbytes.Say("- none"))

						Eventually(sess.Err).Should(gbytes.Say("WARNING:\nGranting role 'viewer' to ALL users. You asked for it!"))
						Consistently(sess.Err).ShouldNot(gbytes.Say("WARNING:\nGranting role 'member' to ALL users. You asked for it!"))
						Consistently(sess.Err).ShouldNot(gbytes.Say("WARNING:\nGranting role 'owner' to ALL users. You asked for it!"))

						Eventually(sess).Should(gbytes.Say(`apply configuration\? \[yN\]: `))
						yes(stdin)

						Eventually(sess).Should(gexec.Exit(0))
					})
				})

				Context("allow-all-users flag provided with other configuration for a given role", func() {
					BeforeEach(func() {
						cmdParams = []string{"-c", "fixtures/team_config_with_conflict.yml"}
						confirmHandlers()
					})

					It("doesn't warn you because noauth has been removed", func() {
						stdin, err := flyCmd.StdinPipe()
						Expect(err).NotTo(HaveOccurred())

						sess, err := gexec.Start(flyCmd, nil, nil)
						Expect(err).ToNot(HaveOccurred())

						Eventually(sess.Out).Should(gbytes.Say("Team Name: venture"))

						Eventually(sess.Out).Should(gbytes.Say("Users \\(owner\\):"))
						Eventually(sess.Out).Should(gbytes.Say("- local:some-admin"))
						Eventually(sess.Out).Should(gbytes.Say("Groups \\(owner\\):"))
						Eventually(sess.Out).Should(gbytes.Say("- none"))

						Consistently(sess.Err).ShouldNot(gbytes.Say("WARNING:\nGranting role 'viewer' to ALL users. You asked for it!"))
						Consistently(sess.Err).ShouldNot(gbytes.Say("WARNING:\nGranting role 'member' to ALL users. You asked for it!"))
						Consistently(sess.Err).ShouldNot(gbytes.Say("WARNING:\nGranting role 'owner' to ALL users. You asked for it!"))

						Eventually(sess).Should(gbytes.Say(`apply configuration\? \[yN\]: `))
						yes(stdin)

						Eventually(sess).Should(gexec.Exit(0))
					})
				})
			})

			Context("Setting local auth", func() {
				BeforeEach(func() {
					cmdParams = []string{"-c", "fixtures/team_config_with_local_auth.yml"}
				})

				It("shows the users configured for local auth for a given role", func() {
					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())

					Eventually(sess.Out).Should(gbytes.Say("Team Name: venture"))

					Eventually(sess.Out).Should(gbytes.Say("Users \\(member\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- local:some-member"))
					Eventually(sess.Out).Should(gbytes.Say("Groups \\(member\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- none"))

					Eventually(sess.Out).Should(gbytes.Say("Users \\(owner\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- local:some-owner"))
					Eventually(sess.Out).Should(gbytes.Say("Groups \\(owner\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- none"))

					Eventually(sess.Out).Should(gbytes.Say("Users \\(viewer\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- local:some-viewer"))
					Eventually(sess.Out).Should(gbytes.Say("Groups \\(viewer\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- none"))

					Eventually(sess).Should(gexec.Exit(1))
				})
			})

			Context("Setting github auth", func() {
				BeforeEach(func() {
					cmdParams = []string{"-c", "fixtures/team_config_with_github_auth.yml"}
				})

				It("shows the users and groups configured for github for a given role", func() {
					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())

					Eventually(sess.Out).Should(gbytes.Say("Team Name: venture"))

					Eventually(sess.Out).Should(gbytes.Say("Users \\(member\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- github:some-user"))
					Eventually(sess.Out).Should(gbytes.Say("Groups \\(member\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- none"))

					Eventually(sess.Out).Should(gbytes.Say("Users \\(owner\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- none"))
					Eventually(sess.Out).Should(gbytes.Say("Groups \\(owner\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- github:some-other-org"))

					Eventually(sess.Out).Should(gbytes.Say("Users \\(viewer\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- github:some-github-user"))
					Eventually(sess.Out).Should(gbytes.Say("Groups \\(viewer\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- github:some-org:some-team"))

					Eventually(sess).Should(gexec.Exit(1))
				})
			})

			Context("Setting cf auth", func() {
				BeforeEach(func() {
					cmdParams = []string{"-c", "fixtures/team_config_with_cf_auth.yml"}
				})

				It("shows the users and groups configured for cf auth for a given role", func() {
					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())

					Eventually(sess.Out).Should(gbytes.Say("Team Name: venture"))

					Eventually(sess.Out).Should(gbytes.Say("Users \\(member\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- cf:some-member"))
					Eventually(sess.Out).Should(gbytes.Say("Groups \\(member\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- none"))

					Eventually(sess.Out).Should(gbytes.Say("Users \\(owner\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- cf:some-admin"))
					Eventually(sess.Out).Should(gbytes.Say("Groups \\(owner\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- cf:some-org"))

					Eventually(sess.Out).Should(gbytes.Say("Users \\(viewer\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- none"))
					Eventually(sess.Out).Should(gbytes.Say("Groups \\(viewer\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- cf:some-org:some-space"))

					Eventually(sess).Should(gexec.Exit(1))
				})
			})

			Context("Setting ldap auth", func() {
				BeforeEach(func() {
					cmdParams = []string{"-c", "fixtures/team_config_with_ldap_auth.yml"}
				})

				It("shows the users and groups configured for ldap auth for a given role", func() {
					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())

					Eventually(sess.Out).Should(gbytes.Say("Team Name: venture"))

					Eventually(sess.Out).Should(gbytes.Say("Users \\(member\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- ldap:some-user"))
					Eventually(sess.Out).Should(gbytes.Say("Groups \\(member\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- none"))

					Eventually(sess.Out).Should(gbytes.Say("Users \\(owner\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- ldap:some-admin"))
					Eventually(sess.Out).Should(gbytes.Say("Groups \\(owner\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- ldap:some-other-group"))

					Eventually(sess.Out).Should(gbytes.Say("Users \\(viewer\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- none"))
					Eventually(sess.Out).Should(gbytes.Say("Groups \\(viewer\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- ldap:some-group"))

					Eventually(sess).Should(gexec.Exit(1))
				})
			})

			Context("Setting generic oauth", func() {
				BeforeEach(func() {
					cmdParams = []string{"-c", "fixtures/team_config_with_generic_oauth.yml"}
				})

				It("shows the groups configured for generic oauth for a given role", func() {
					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())

					Eventually(sess.Out).Should(gbytes.Say("Team Name: venture"))

					Eventually(sess.Out).Should(gbytes.Say("Users \\(member\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- oauth:some-user"))
					Eventually(sess.Out).Should(gbytes.Say("Groups \\(member\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- none"))

					Eventually(sess.Out).Should(gbytes.Say("Users \\(owner\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- oauth:some-admin"))
					Eventually(sess.Out).Should(gbytes.Say("Groups \\(owner\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- oauth:some-other-group"))

					Eventually(sess.Out).Should(gbytes.Say("Users \\(viewer\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- none"))
					Eventually(sess.Out).Should(gbytes.Say("Groups \\(viewer\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- oauth:some-group"))

					Eventually(sess).Should(gexec.Exit(1))
				})
			})
		})

		Describe("confirmation", func() {
			BeforeEach(func() {
				cmdParams = []string{"-c", "fixtures/team_config.yml"}
			})

			Context("when the user presses y/yes", func() {
				BeforeEach(func() {
					confirmHandlers()
				})

				It("exits 0", func() {
					stdin, err := flyCmd.StdinPipe()
					Expect(err).NotTo(HaveOccurred())

					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())

					Eventually(sess).Should(gbytes.Say(`apply configuration\? \[yN\]: `))
					yes(stdin)

					Eventually(sess).Should(gexec.Exit(0))
				})
			})

			Context("when the user presses n/no", func() {
				It("exits 1", func() {
					stdin, err := flyCmd.StdinPipe()
					Expect(err).NotTo(HaveOccurred())

					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())

					Eventually(sess).Should(gbytes.Say(`apply configuration\? \[yN\]: `))
					no(stdin)

					Eventually(sess.Err).Should(gbytes.Say("bailing out"))
					Eventually(sess).Should(gexec.Exit(1))
				})
			})
		})

		Describe("sending", func() {
			BeforeEach(func() {
				cmdParams = []string{"-c", "fixtures/team_config_mixed.yml"}

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/api/v1/teams/venture"),
						ghttp.VerifyJSON(`{
							"auth": {
								"owner":{
									"users": [
										"github:some-github-user",
										"local:some-admin"
									],
									"groups": [
										"oauth:some-oauth-group"
									]
								},
								"member":{
									"users": [
										"local:some-user"
									],
									"groups": []
								},
								"viewer":{
									"users": [],
									"groups": []
								}
							}
						}`),
						ghttp.RespondWithJSONEncoded(http.StatusCreated, atc.Team{
							Name: "venture",
							ID:   8,
						}),
					),
				)
			})

			It("sends the expected request", func() {
				stdin, err := flyCmd.StdinPipe()
				Expect(err).NotTo(HaveOccurred())

				sess, err := gexec.Start(flyCmd, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				Eventually(sess).Should(gbytes.Say(`apply configuration\? \[yN\]: `))
				yes(stdin)

				Eventually(sess).Should(gexec.Exit(0))
			})

			It("Outputs created for new team", func() {
				stdin, err := flyCmd.StdinPipe()
				Expect(err).NotTo(HaveOccurred())

				sess, err := gexec.Start(flyCmd, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				Eventually(sess).Should(gbytes.Say(`apply configuration\? \[yN\]: `))
				yes(stdin)

				Eventually(sess.Out).Should(gbytes.Say("team created"))

				Eventually(sess).Should(gexec.Exit(0))
			})
		})

		Describe("handling server response", func() {
			BeforeEach(func() {
				cmdParams = []string{"-c", "fixtures/team_config_mixed.yml"}
			})

			Context("when the server returns 500", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("PUT", "/api/v1/teams/venture"),
							ghttp.VerifyJSON(`{
							"auth": {
								"owner":{
									"users": [
										"github:some-github-user",
										"local:some-admin"
									],
									"groups": [
										"oauth:some-oauth-group"
									]
								},
								"member":{
									"users": [
										"local:some-user"
									],
									"groups": []
								},
								"viewer":{
									"users": [],
									"groups": []
								}
							}
						}`),
							ghttp.RespondWith(http.StatusInternalServerError, "sorry bro"),
						),
					)
				})

				It("reports the error", func() {
					stdin, err := flyCmd.StdinPipe()
					Expect(err).NotTo(HaveOccurred())

					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())

					Eventually(sess).Should(gbytes.Say(`apply configuration\? \[yN\]: `))
					yes(stdin)

					Eventually(sess.Err).Should(gbytes.Say("sorry bro"))

					Eventually(sess).Should(gexec.Exit(1))
				})
			})
		})
	})

	Context("using command line args", func() {
		Describe("flag validation", func() {
			Describe("no auth", func() {
				Context("auth flag not provided", func() {
					BeforeEach(func() {
						cmdParams = []string{}
					})

					It("returns an error", func() {
						sess, err := gexec.Start(flyCmd, nil, nil)
						Expect(err).ToNot(HaveOccurred())
						Eventually(sess.Err).Should(gbytes.Say("You have not provided a whitelist of users or groups. To continue, run:"))
						Eventually(sess.Err).Should(gbytes.Say("fly -t testserver set-team -n venture --allow-all-users"))
						Eventually(sess.Err).Should(gbytes.Say("This will allow team access to all logged in users in the system."))
						Eventually(sess).Should(gexec.Exit(1))
					})
				})
			})
		})

		Describe("Display", func() {
			Context("Setting no auth", func() {
				Context("allow-all-users flag provided", func() {
					BeforeEach(func() {
						cmdParams = []string{"--allow-all-users"}
						confirmHandlers()
					})

					It("show a warning about creating unauthenticated team", func() {
						stdin, err := flyCmd.StdinPipe()
						Expect(err).NotTo(HaveOccurred())

						sess, err := gexec.Start(flyCmd, nil, nil)
						Expect(err).ToNot(HaveOccurred())

						Eventually(sess.Out).Should(gbytes.Say("Team Name: venture"))
						Eventually(sess.Out).Should(gbytes.Say("Users \\(owner\\):"))
						Eventually(sess.Out).Should(gbytes.Say("- none"))
						Eventually(sess.Out).Should(gbytes.Say("Groups \\(owner\\):"))
						Eventually(sess.Out).Should(gbytes.Say("- none"))

						Eventually(sess.Err).Should(gbytes.Say("WARNING:\nGranting role 'owner' to ALL users. You asked for it!"))

						Eventually(sess).Should(gbytes.Say(`apply configuration\? \[yN\]: `))
						yes(stdin)

						Eventually(sess).Should(gexec.Exit(0))
					})
				})

				Context("allow-all-users flag provided with other configuration", func() {
					BeforeEach(func() {
						cmdParams = []string{"--local-user", "brock-samson", "--allow-all-users"}
						confirmHandlers()
					})

					It("doesn't warn you because noauth has been removed", func() {
						stdin, err := flyCmd.StdinPipe()
						Expect(err).NotTo(HaveOccurred())

						sess, err := gexec.Start(flyCmd, nil, nil)
						Expect(err).ToNot(HaveOccurred())

						Eventually(sess.Out).Should(gbytes.Say("Team Name: venture"))
						Eventually(sess.Out).Should(gbytes.Say("Users \\(owner\\):"))
						Eventually(sess.Out).Should(gbytes.Say("- local:brock-samson"))
						Eventually(sess.Out).Should(gbytes.Say("Groups \\(owner\\):"))
						Eventually(sess.Out).Should(gbytes.Say("- none"))

						Consistently(sess.Err).ShouldNot(gbytes.Say("WARNING:\nGranting role 'owner' to ALL users. You asked for it!"))

						Eventually(sess).Should(gbytes.Say(`apply configuration\? \[yN\]: `))
						yes(stdin)

						Eventually(sess).Should(gexec.Exit(0))
					})
				})
			})

			Context("Setting local auth", func() {
				BeforeEach(func() {
					cmdParams = []string{"--local-user", "brock-samson"}
				})

				It("shows the users configured for local auth", func() {
					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())

					Eventually(sess.Out).Should(gbytes.Say("Team Name: venture"))
					Eventually(sess.Out).Should(gbytes.Say("Users \\(owner\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- local:brock-samson"))
					Eventually(sess.Out).Should(gbytes.Say("Groups \\(owner\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- none"))

					Eventually(sess).Should(gexec.Exit(1))
				})
			})

			Context("Setting github auth", func() {
				BeforeEach(func() {
					cmdParams = []string{"--github-org", "my-org", "--github-team", "samson-org:samson-team", "--github-user", "samsonite"}
				})

				It("shows the users and groups configured for github", func() {
					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())

					Eventually(sess.Out).Should(gbytes.Say("Team Name: venture"))
					Eventually(sess.Out).Should(gbytes.Say("Users \\(owner\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- github:samsonite"))
					Eventually(sess.Out).Should(gbytes.Say("Groups \\(owner\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- github:my-org"))
					Eventually(sess.Out).Should(gbytes.Say("- github:samson-org:samson-team"))

					Eventually(sess).Should(gexec.Exit(1))
				})
			})

			Context("Setting cf auth", func() {
				BeforeEach(func() {
					cmdParams = []string{"--cf-org", "myorg-1", "--cf-space", "myorg-2:myspace", "--cf-user", "my-username", "--cf-space-guid", "myspace-guid"}
				})

				It("shows the users and groups configured for cf auth", func() {
					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())

					Eventually(sess.Out).Should(gbytes.Say("Team Name: venture"))
					Eventually(sess.Out).Should(gbytes.Say("Users \\(owner\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- cf:my-username"))
					Eventually(sess.Out).Should(gbytes.Say("Groups \\(owner\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- cf:myorg-1"))
					Eventually(sess.Out).Should(gbytes.Say("- cf:myorg-2:myspace"))
					Eventually(sess.Out).Should(gbytes.Say("- cf:myspace-guid"))

					Eventually(sess).Should(gexec.Exit(1))
				})
			})

			Context("Setting ldap auth", func() {
				BeforeEach(func() {
					cmdParams = []string{"--ldap-group", "my-group", "--ldap-user", "my-username"}
				})

				It("shows the users and groups configured for ldap auth", func() {
					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())

					Eventually(sess.Out).Should(gbytes.Say("Team Name: venture"))
					Eventually(sess.Out).Should(gbytes.Say("Users \\(owner\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- ldap:my-username"))
					Eventually(sess.Out).Should(gbytes.Say("Groups \\(owner\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- ldap:my-group"))

					Eventually(sess).Should(gexec.Exit(1))
				})
			})

			Context("Setting generic oauth", func() {
				BeforeEach(func() {
					cmdParams = []string{
						"--oauth-group", "cool-scope-name",
					}
				})

				It("shows the groups configured for generic oauth", func() {
					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())

					Eventually(sess.Out).Should(gbytes.Say("Team Name: venture"))
					Eventually(sess.Out).Should(gbytes.Say("Users \\(owner\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- none"))
					Eventually(sess.Out).Should(gbytes.Say("Groups \\(owner\\):"))
					Eventually(sess.Out).Should(gbytes.Say("- oauth:cool-scope-name"))

					Eventually(sess).Should(gexec.Exit(1))
				})
			})
		})

		Describe("confirmation", func() {
			BeforeEach(func() {
				cmdParams = []string{"--local-user", "brock-samson"}
			})

			Context("when the user presses y/yes", func() {
				BeforeEach(func() {
					confirmHandlers()
				})

				It("exits 0", func() {
					stdin, err := flyCmd.StdinPipe()
					Expect(err).NotTo(HaveOccurred())

					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())

					Eventually(sess).Should(gbytes.Say(`apply configuration\? \[yN\]: `))
					yes(stdin)

					Eventually(sess).Should(gexec.Exit(0))
				})
			})

			Context("when the user presses n/no", func() {
				It("exits 1", func() {
					stdin, err := flyCmd.StdinPipe()
					Expect(err).NotTo(HaveOccurred())

					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())

					Eventually(sess).Should(gbytes.Say(`apply configuration\? \[yN\]: `))
					no(stdin)

					Eventually(sess.Err).Should(gbytes.Say("bailing out"))
					Eventually(sess).Should(gexec.Exit(1))
				})
			})
		})

		Describe("sending", func() {
			BeforeEach(func() {
				cmdParams = []string{
					"--local-user", "brock-obama",
					"--github-org", "obama-org",
					"--github-team", "samson-org:venture-team",
					"--github-user", "lisa",
					"--github-user", "frank",
				}

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/api/v1/teams/venture"),
						ghttp.VerifyJSON(`{
							"auth": {
								"owner":{
									"users": [
										"github:lisa",
										"github:frank",
										"local:brock-obama"
									],
									"groups": [
										"github:obama-org",
										"github:samson-org:venture-team"
									]
								}
							}
						}`),
						ghttp.RespondWithJSONEncoded(http.StatusCreated, atc.Team{
							Name: "venture",
							ID:   8,
						}),
					),
				)
			})

			It("sends the expected request", func() {
				stdin, err := flyCmd.StdinPipe()
				Expect(err).NotTo(HaveOccurred())

				sess, err := gexec.Start(flyCmd, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				Eventually(sess).Should(gbytes.Say(`apply configuration\? \[yN\]: `))
				yes(stdin)

				Eventually(sess).Should(gexec.Exit(0))
			})

			It("Outputs created for new team", func() {
				stdin, err := flyCmd.StdinPipe()
				Expect(err).NotTo(HaveOccurred())

				sess, err := gexec.Start(flyCmd, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				Eventually(sess).Should(gbytes.Say(`apply configuration\? \[yN\]: `))
				yes(stdin)

				Eventually(sess.Out).Should(gbytes.Say("team created"))

				Eventually(sess).Should(gexec.Exit(0))
			})
		})

		Describe("handling server response", func() {
			BeforeEach(func() {
				cmdParams = []string{"--local-user", "brock-obama"}
			})

			Context("when the server returns 500", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("PUT", "/api/v1/teams/venture"),
							ghttp.VerifyJSON(`{
							"auth": {
								"owner":{
									"users": [
										"local:brock-obama"
									],
									"groups": []
								}
							}
						}`),
							ghttp.RespondWith(http.StatusInternalServerError, "sorry bro"),
						),
					)
				})

				It("reports the error", func() {
					stdin, err := flyCmd.StdinPipe()
					Expect(err).NotTo(HaveOccurred())

					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())

					Eventually(sess).Should(gbytes.Say(`apply configuration\? \[yN\]: `))
					yes(stdin)

					Eventually(sess.Err).Should(gbytes.Say("sorry bro"))

					Eventually(sess).Should(gexec.Exit(1))
				})
			})
		})
	})
})

func confirmHandlers() {
	atcServer.AppendHandlers(
		ghttp.CombineHandlers(
			ghttp.VerifyRequest("PUT", "/api/v1/teams/venture"),
			ghttp.RespondWithJSONEncoded(http.StatusCreated, atc.Team{
				Name: "venture",
				ID:   8,
			}),
		),
	)
}
