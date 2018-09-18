package integration_test

import (
	"fmt"
	"io"
	"net/http"
	"os/exec"

	"github.com/concourse/atc"
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
					Eventually(sess.Out).Should(gbytes.Say("Users:"))
					Eventually(sess.Out).Should(gbytes.Say("- none"))
					Eventually(sess.Out).Should(gbytes.Say("Groups:"))
					Eventually(sess.Out).Should(gbytes.Say("- none"))

					Eventually(sess.Err).Should(gbytes.Say("WARNING:\nAllowing all logged in users. You asked for it!"))

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
					Eventually(sess.Out).Should(gbytes.Say("Users:"))
					Eventually(sess.Out).Should(gbytes.Say("- local:brock-samson"))
					Eventually(sess.Out).Should(gbytes.Say("Groups:"))
					Eventually(sess.Out).Should(gbytes.Say("- none"))

					Consistently(sess.Err).ShouldNot(gbytes.Say("WARNING:\nAllowing all logged in users. You asked for it!"))

					Eventually(sess).Should(gbytes.Say(`apply configuration\? \[yN\]: `))
					yes(stdin)

					Eventually(sess).Should(gexec.Exit(0))
				})
			})
		})

		Context("Setting basic auth", func() {
			BeforeEach(func() {
				cmdParams = []string{"--local-user", "brock-samson"}
			})

			It("shows the users configured for basic auth", func() {
				sess, err := gexec.Start(flyCmd, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				Eventually(sess.Out).Should(gbytes.Say("Team Name: venture"))
				Eventually(sess.Out).Should(gbytes.Say("Users:"))
				Eventually(sess.Out).Should(gbytes.Say("- local:brock-samson"))
				Eventually(sess.Out).Should(gbytes.Say("Groups:"))
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
				Eventually(sess.Out).Should(gbytes.Say("Users:"))
				Eventually(sess.Out).Should(gbytes.Say("- github:samsonite"))
				Eventually(sess.Out).Should(gbytes.Say("Groups:"))
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
				Eventually(sess.Out).Should(gbytes.Say("Users:"))
				Eventually(sess.Out).Should(gbytes.Say("- cf:my-username"))
				Eventually(sess.Out).Should(gbytes.Say("Groups:"))
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
				Eventually(sess.Out).Should(gbytes.Say("Users:"))
				Eventually(sess.Out).Should(gbytes.Say("- ldap:my-username"))
				Eventually(sess.Out).Should(gbytes.Say("Groups:"))
				Eventually(sess.Out).Should(gbytes.Say("- ldap:my-group"))

				Eventually(sess).Should(gexec.Exit(1))
			})
		})

		XContext("Setting generic oauth", func() {
			BeforeEach(func() {
				cmdParams = []string{
					"--oauth-group", "cool-scope-name",
				}
			})

			It("shows the groups configured for generic oauth", func() {
				sess, err := gexec.Start(flyCmd, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				Eventually(sess.Out).Should(gbytes.Say("Team Name: venture"))
				Eventually(sess.Out).Should(gbytes.Say("Users:"))
				Eventually(sess.Out).Should(gbytes.Say("- none"))
				Eventually(sess.Out).Should(gbytes.Say("Groups:"))
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
								"users": [
									"local:brock-obama"
								],
								"groups": []
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
