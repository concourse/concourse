package integration_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
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
		Describe("basic auth", func() {
			Context("username omitted", func() {
				BeforeEach(func() {
					cmdParams = []string{"--basic-auth-password", "brock123"}
				})

				It("returns an error", func() {
					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())
					Eventually(sess.Err).Should(gbytes.Say("Both username and password are required for basic auth."))
					Eventually(sess).Should(gexec.Exit(1))
				})
			})

			Context("password omitted", func() {
				BeforeEach(func() {
					cmdParams = []string{"--basic-auth-username", "brock samson"}
				})

				It("returns an error", func() {
					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())
					Eventually(sess.Err).Should(gbytes.Say("Both username and password are required for basic auth."))
					Eventually(sess).Should(gexec.Exit(1))
				})
			})
		})

		Describe("github auth", func() {
			Context("ClientID omitted", func() {
				BeforeEach(func() {
					cmdParams = []string{"--github-auth-client-secret", "brock123"}
				})

				It("returns an error", func() {
					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())
					Eventually(sess.Err).Should(gbytes.Say("Both client-id and client-secret are required for github-auth."))
					Eventually(sess).Should(gexec.Exit(1))
				})
			})

			Context("ClientSecret omitted", func() {
				BeforeEach(func() {
					cmdParams = []string{"--github-auth-client-id", "Brock Samson"}
				})

				It("returns an error", func() {
					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())
					Eventually(sess.Err).Should(gbytes.Say("Both client-id and client-secret are required for github-auth."))
					Eventually(sess).Should(gexec.Exit(1))
				})
			})

			Context("ClientID and ClientSecret omitted", func() {
				BeforeEach(func() {
					cmdParams = []string{"--github-auth-organization", "Samson, Inc"}
				})

				It("returns an error", func() {
					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())
					Eventually(sess.Err).Should(gbytes.Say("Both client-id and client-secret are required for github-auth."))
					Eventually(sess).Should(gexec.Exit(1))
				})
			})

			Context("organizations, teams, and users are omitted", func() {
				BeforeEach(func() {
					cmdParams = []string{"--github-auth-client-id", "Brock Samson", "--github-auth-client-secret", "brock123"}
				})

				It("returns an error", func() {
					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())
					Eventually(sess.Err).Should(gbytes.Say("At least one of the following is required for github-auth: organizations, teams, users"))
					Eventually(sess).Should(gexec.Exit(1))
				})
			})
		})

		Describe("uaa auth", func() {
			Context("ClientID omitted", func() {
				BeforeEach(func() {
					cmdParams = []string{"--uaa-auth-client-secret", "brock123"}
				})

				It("returns an error", func() {
					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())
					Eventually(sess.Err).Should(gbytes.Say("Both client-id and client-secret are required for uaa-auth."))
					Eventually(sess).Should(gexec.Exit(1))
				})
			})

			Context("ClientSecret omitted", func() {
				BeforeEach(func() {
					cmdParams = []string{"--uaa-auth-client-id", "Brock Samson"}
				})

				It("returns an error", func() {
					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())
					Eventually(sess.Err).Should(gbytes.Say("Both client-id and client-secret are required for uaa-auth."))
					Eventually(sess).Should(gexec.Exit(1))
				})
			})

			Context("Space omitted", func() {
				BeforeEach(func() {
					cmdParams = []string{
						"--uaa-auth-client-id", "Brock Samson",
						"--uaa-auth-client-secret", "brock123",
					}
				})

				It("returns an error", func() {
					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())
					Eventually(sess.Err).Should(gbytes.Say("cf-space is required for uaa-auth."))
					Eventually(sess).Should(gexec.Exit(1))
				})
			})

			Context("TokenURL omitted", func() {
				BeforeEach(func() {
					cmdParams = []string{
						"--uaa-auth-client-id", "Brock Samson",
						"--uaa-auth-client-secret", "brock123",
						"--uaa-auth-cf-space", "myspace",
						"--uaa-auth-auth-url", "http://auth.example.url",
						"--uaa-auth-cf-url", "http://api.example.url",
					}
				})

				It("returns an error", func() {
					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())
					Eventually(sess.Err).Should(gbytes.Say("auth-url, token-url and cf-url are required for uaa-auth."))
					Eventually(sess).Should(gexec.Exit(1))
				})
			})

			Context("AuthUrl omitted", func() {
				BeforeEach(func() {
					cmdParams = []string{
						"--uaa-auth-client-id", "Brock Samson",
						"--uaa-auth-client-secret", "brock123",
						"--uaa-auth-cf-space", "myspace",
						"--uaa-auth-token-url", "http://token.example.url",
						"--uaa-auth-cf-url", "http://api.example.url",
					}
				})

				It("returns an error", func() {
					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())
					Eventually(sess.Err).Should(gbytes.Say("auth-url, token-url and cf-url are required for uaa-auth."))
					Eventually(sess).Should(gexec.Exit(1))
				})
			})

			Context("ApiURL omitted", func() {
				BeforeEach(func() {
					cmdParams = []string{
						"--uaa-auth-client-id", "Brock Samson",
						"--uaa-auth-client-secret", "brock123",
						"--uaa-auth-cf-space", "myspace",
						"--uaa-auth-auth-url", "http://auth.example.url",
						"--uaa-auth-token-url", "http://token.example.url",
					}
				})

				It("returns an error", func() {
					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())
					Eventually(sess.Err).Should(gbytes.Say("auth-url, token-url and cf-url are required for uaa-auth."))
					Eventually(sess).Should(gexec.Exit(1))
				})
			})
		})
	})

	Describe("Display", func() {
		Context("Setting basic auth", func() {
			BeforeEach(func() {
				cmdParams = []string{"--basic-auth-username", "brock samson", "--basic-auth-password", "brock123"}
			})

			It("says 'enabled' to setting basic auth and 'disabled' to the rest auths", func() {
				sess, err := gexec.Start(flyCmd, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				Eventually(sess.Out).Should(gbytes.Say("Team Name: venture"))
				Eventually(sess.Out).Should(gbytes.Say("Basic Auth: enabled"))
				Eventually(sess.Out).Should(gbytes.Say("GitHub Auth: disabled"))
				Eventually(sess.Out).Should(gbytes.Say("UAA Auth: disabled"))

				Eventually(sess).Should(gexec.Exit(1))
			})
		})

		Context("Setting github auth", func() {
			BeforeEach(func() {
				cmdParams = []string{"--github-auth-client-id", "Brock Samson", "--github-auth-client-secret", "brock123", "--github-auth-organization", "Samson, Inc"}
			})

			It("says 'disabled' to setting basic auth and uaa auth and 'enabled' to github auth", func() {
				sess, err := gexec.Start(flyCmd, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				Eventually(sess.Out).Should(gbytes.Say("Team Name: venture"))
				Eventually(sess.Out).Should(gbytes.Say("Basic Auth: disabled"))
				Eventually(sess.Out).Should(gbytes.Say("GitHub Auth: enabled"))
				Eventually(sess.Out).Should(gbytes.Say("UAA Auth: disabled"))

				Eventually(sess).Should(gexec.Exit(1))
			})
		})

		Context("Setting uaa auth", func() {
			var cfCertFile *os.File

			BeforeEach(func() {
				var err error
				cfCertFile, err = ioutil.TempFile("", "test-cf-cert")
				Expect(err).NotTo(HaveOccurred())

				err = cfCertFile.Close()
				Expect(err).NotTo(HaveOccurred())

				cmdParams = []string{
					"--uaa-auth-client-id", "Brock Samson",
					"--uaa-auth-client-secret", "brock123",
					"--uaa-auth-cf-space", "myspace",
					"--uaa-auth-auth-url", "http://auth.example.url",
					"--uaa-auth-token-url", "http://token.example.url",
					"--uaa-auth-cf-url", "http://api.example.url",
					"--uaa-auth-cf-ca-cert", cfCertFile.Name(),
				}
			})

			AfterEach(func() {
				err := os.RemoveAll(cfCertFile.Name())
				Expect(err).NotTo(HaveOccurred())
			})

			It("says 'disabled' to setting basic auth and github auth and 'enabled' to uaa auth", func() {
				sess, err := gexec.Start(flyCmd, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				Eventually(sess.Out).Should(gbytes.Say("Team Name: venture"))
				Eventually(sess.Out).Should(gbytes.Say("Basic Auth: disabled"))
				Eventually(sess.Out).Should(gbytes.Say("GitHub Auth: disabled"))
				Eventually(sess.Out).Should(gbytes.Say("UAA Auth: enabled"))

				Eventually(sess).Should(gexec.Exit(1))
			})
		})
	})

	Describe("confirmation", func() {
		BeforeEach(func() {
			cmdParams = []string{"--basic-auth-username", "brock samson", "--basic-auth-password", "brock123"}
		})

		Context("when the user presses y/yes", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/api/v1/teams/venture"),
						ghttp.RespondWithJSONEncoded(http.StatusCreated, atc.Team{
							Name: "venture",
							ID:   8,
						}),
					),
				)
			})

			It("exits 0", func() {
				stdin, err := flyCmd.StdinPipe()
				Expect(err).NotTo(HaveOccurred())

				sess, err := gexec.Start(flyCmd, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				Eventually(sess.Out).Should(gbytes.Say("Team Name: venture"))
				Eventually(sess.Out).Should(gbytes.Say("Basic Auth: enabled"))
				Eventually(sess.Out).Should(gbytes.Say("GitHub Auth: disabled"))
				Eventually(sess.Out).Should(gbytes.Say("UAA Auth: disabled"))

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

				Eventually(sess.Out).Should(gbytes.Say("Team Name: venture"))
				Eventually(sess.Out).Should(gbytes.Say("Basic Auth: enabled"))
				Eventually(sess.Out).Should(gbytes.Say("GitHub Auth: disabled"))
				Eventually(sess.Out).Should(gbytes.Say("UAA Auth: disabled"))

				Eventually(sess).Should(gbytes.Say(`apply configuration\? \[yN\]: `))
				no(stdin)

				Eventually(sess.Err).Should(gbytes.Say("bailing out"))
				Eventually(sess).Should(gexec.Exit(1))
			})
		})
	})

	Describe("sending", func() {
		Context("with CA Cert", func() {
			var cfCertFilePath string

			BeforeEach(func() {
				cfCertFile, err := ioutil.TempFile("", "test-cf-cert")
				Expect(err).NotTo(HaveOccurred())

				_, err = cfCertFile.WriteString("cf-cert-contents")
				Expect(err).NotTo(HaveOccurred())

				err = cfCertFile.Close()
				Expect(err).NotTo(HaveOccurred())

				cfCertFilePath = cfCertFile.Name()

				cmdParams = []string{
					"--basic-auth-username", "brock obama",
					"--basic-auth-password", "brock123",
					"--github-auth-client-id", "barack samson",
					"--github-auth-client-secret", "barack123",
					"--github-auth-organization", "Obama, Inc",
					"--github-auth-organization", "Samson, Inc",
					"--github-auth-team", "Venture, Inc/venture-devs",
					"--github-auth-user", "lisa",
					"--github-auth-user", "frank",
					"--uaa-auth-client-id", "barack samson",
					"--uaa-auth-client-secret", "barack123",
					"--uaa-auth-cf-space", "Obama, Inc",
					"--uaa-auth-cf-space", "Samson, Inc",
					"--uaa-auth-auth-url", "http://uaa.auth.url",
					"--uaa-auth-token-url", "http://uaa.token.url",
					"--uaa-auth-cf-url", "http://cf.url",
					"--uaa-auth-cf-ca-cert", cfCertFilePath,
				}
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/api/v1/teams/venture"),
						ghttp.VerifyJSON(`{
							"basic_auth": {
								"basic_auth_username": "brock obama",
								"basic_auth_password": "brock123"
							},
							"github_auth": {
								"client_id": "barack samson",
								"client_secret": "barack123",
								"organizations": ["Obama, Inc", "Samson, Inc"],
								"teams": [{"organization_name": "Venture, Inc", "team_name": "venture-devs"}],
								"users": ["lisa", "frank"]
							},
							"uaa_auth": {
								"client_id": "barack samson",
								"client_secret": "barack123",
								"auth_url": "http://uaa.auth.url",
								"token_url": "http://uaa.token.url",
								"cf_spaces": ["Obama, Inc", "Samson, Inc"],
								"cf_url": "http://cf.url",
								"cf_ca_cert": "cf-cert-contents"
							}
						}`),
						ghttp.RespondWithJSONEncoded(http.StatusCreated, atc.Team{
							Name: "venture",
							ID:   8,
						}),
					),
				)
			})

			AfterEach(func() {
				err := os.RemoveAll(cfCertFilePath)
				Expect(err).NotTo(HaveOccurred())
			})

			It("sends the expected request", func() {
				stdin, err := flyCmd.StdinPipe()
				Expect(err).NotTo(HaveOccurred())

				sess, err := gexec.Start(flyCmd, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				Eventually(sess.Out).Should(gbytes.Say("Team Name: venture"))
				Eventually(sess.Out).Should(gbytes.Say("Basic Auth: enabled"))
				Eventually(sess.Out).Should(gbytes.Say("GitHub Auth: enabled"))
				Eventually(sess.Out).Should(gbytes.Say("UAA Auth: enabled"))

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

		Context("without CA Cert", func() {
			var cfCertFilePath string

			BeforeEach(func() {
				cfCertFile, err := ioutil.TempFile("", "test-cf-cert")
				Expect(err).NotTo(HaveOccurred())

				_, err = cfCertFile.WriteString("cf-cert-contents")
				Expect(err).NotTo(HaveOccurred())

				err = cfCertFile.Close()
				Expect(err).NotTo(HaveOccurred())

				cfCertFilePath = cfCertFile.Name()

				cmdParams = []string{
					"--basic-auth-username", "brock obama",
					"--basic-auth-password", "brock123",
					"--github-auth-client-id", "barack samson",
					"--github-auth-client-secret", "barack123",
					"--github-auth-organization", "Obama, Inc",
					"--github-auth-organization", "Samson, Inc",
					"--github-auth-team", "Venture, Inc/venture-devs",
					"--github-auth-user", "lisa",
					"--github-auth-user", "frank",
					"--uaa-auth-client-id", "barack samson",
					"--uaa-auth-client-secret", "barack123",
					"--uaa-auth-cf-space", "Obama, Inc",
					"--uaa-auth-cf-space", "Samson, Inc",
					"--uaa-auth-auth-url", "http://uaa.auth.url",
					"--uaa-auth-token-url", "http://uaa.token.url",
					"--uaa-auth-cf-url", "http://cf.url",
				}
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/api/v1/teams/venture"),
						ghttp.VerifyJSON(`{
							"basic_auth": {
								"basic_auth_username": "brock obama",
								"basic_auth_password": "brock123"
							},
							"github_auth": {
								"client_id": "barack samson",
								"client_secret": "barack123",
								"organizations": ["Obama, Inc", "Samson, Inc"],
								"teams": [{"organization_name": "Venture, Inc", "team_name": "venture-devs"}],
								"users": ["lisa", "frank"]
							},
							"uaa_auth": {
								"client_id": "barack samson",
								"client_secret": "barack123",
								"auth_url": "http://uaa.auth.url",
								"token_url": "http://uaa.token.url",
								"cf_spaces": ["Obama, Inc", "Samson, Inc"],
								"cf_url": "http://cf.url",
								"cf_ca_cert": ""
							}
						}`),
						ghttp.RespondWithJSONEncoded(http.StatusCreated, atc.Team{
							Name: "venture",
							ID:   8,
						}),
					),
				)
			})

			AfterEach(func() {
				err := os.RemoveAll(cfCertFilePath)
				Expect(err).NotTo(HaveOccurred())
			})

			It("sends the expected request", func() {
				stdin, err := flyCmd.StdinPipe()
				Expect(err).NotTo(HaveOccurred())

				sess, err := gexec.Start(flyCmd, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				Eventually(sess.Out).Should(gbytes.Say("Team Name: venture"))
				Eventually(sess.Out).Should(gbytes.Say("Basic Auth: enabled"))
				Eventually(sess.Out).Should(gbytes.Say("GitHub Auth: enabled"))
				Eventually(sess.Out).Should(gbytes.Say("UAA Auth: enabled"))

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
	})

	Describe("handling server response", func() {
		BeforeEach(func() {
			cmdParams = []string{
				"--basic-auth-username", "brock obama",
				"--basic-auth-password", "brock123",
			}
		})

		Context("when the server returns 500", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/api/v1/teams/venture"),
						ghttp.VerifyJSON(`{
							"basic_auth": {
								"basic_auth_username": "brock obama",
								"basic_auth_password": "brock123"
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
