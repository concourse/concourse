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

		Describe("no auth", func() {
			Context("auth flag not provided", func() {
				BeforeEach(func() {
					cmdParams = []string{}
				})

				It("returns an error", func() {
					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())
					Eventually(sess.Err).Should(gbytes.Say("no auth methods configured! to continue, run:"))
					Eventually(sess.Err).Should(gbytes.Say("fly -t testserver set-team -n venture --no-really-i-dont-want-any-auth"))
					Eventually(sess.Err).Should(gbytes.Say("this will leave the team open to anyone to mess with!"))
					Eventually(sess).Should(gexec.Exit(1))
				})
			})

			Context("no really I don't want any auth flag provided", func() {
				BeforeEach(func() {
					cmdParams = []string{"--no-really-i-dont-want-any-auth"}
					confirmHandlers()
				})

				It("show a warning about creating unauthenticated team", func() {
					stdin, err := flyCmd.StdinPipe()
					Expect(err).NotTo(HaveOccurred())

					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())
					Eventually(sess.Err).Should(gbytes.Say("WARNING:\nno auth methods configured. you asked for it!"))

					Eventually(sess.Out).Should(gbytes.Say("Team Name: venture"))
					Eventually(sess.Out).Should(gbytes.Say("Basic Auth: disabled"))
					Eventually(sess.Out).Should(gbytes.Say("GitHub Auth: disabled"))
					Eventually(sess.Out).Should(gbytes.Say("UAA Auth: disabled"))

					Eventually(sess).Should(gbytes.Say(`apply configuration\? \[yN\]: `))
					yes(stdin)

					Eventually(sess).Should(gexec.Exit(0))

				})
			})

		})

		Describe("basic auth", func() {
			Context("username omitted", func() {
				BeforeEach(func() {
					cmdParams = []string{"--basic-auth-password", "brock123"}
				})

				It("returns an error", func() {
					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())
					Eventually(sess.Err).Should(gbytes.Say("must specify --basic-auth-username to use basic auth."))
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
					Eventually(sess.Err).Should(gbytes.Say("must specify --basic-auth-password to use basic auth."))
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
					Eventually(sess.Err).Should(gbytes.Say("must specify --github-auth-client-id and --github-auth-client-secret to use GitHub OAuth."))
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
					Eventually(sess.Err).Should(gbytes.Say("must specify --github-auth-client-id and --github-auth-client-secret to use GitHub OAuth."))
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
					Eventually(sess.Err).Should(gbytes.Say("must specify --github-auth-client-id and --github-auth-client-secret to use GitHub OAuth."))
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
					Eventually(sess.Err).Should(gbytes.Say("at least one of the following is required for github-auth: organizations, teams, users."))
					Eventually(sess).Should(gexec.Exit(1))
				})
			})
		})

		Describe("gitlab auth", func() {
			Context("ClientID omitted", func() {
				BeforeEach(func() {
					cmdParams = []string{"--gitlab-auth-client-secret", "brock123"}
				})

				It("returns an error", func() {
					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())
					Eventually(sess.Err).Should(gbytes.Say("must specify --gitlab-auth-client-id and --gitlab-auth-client-secret to use GitLab OAuth."))
					Eventually(sess).Should(gexec.Exit(1))
				})
			})

			Context("ClientSecret omitted", func() {
				BeforeEach(func() {
					cmdParams = []string{"--gitlab-auth-client-id", "Brock Samson"}
				})

				It("returns an error", func() {
					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())
					Eventually(sess.Err).Should(gbytes.Say("must specify --gitlab-auth-client-id and --gitlab-auth-client-secret to use GitLab OAuth."))
					Eventually(sess).Should(gexec.Exit(1))
				})
			})

			Context("ClientID and ClientSecret omitted", func() {
				BeforeEach(func() {
					cmdParams = []string{"--gitlab-auth-group", "my-gitlab-group"}
				})

				It("returns an error", func() {
					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())
					Eventually(sess.Err).Should(gbytes.Say("must specify --gitlab-auth-client-id and --gitlab-auth-client-secret to use GitLab OAuth."))
					Eventually(sess).Should(gexec.Exit(1))
				})
			})

			Context("group is omitted", func() {
				BeforeEach(func() {
					cmdParams = []string{"--gitlab-auth-client-id", "Brock Samson", "--gitlab-auth-client-secret", "brock123"}
				})

				It("returns an error", func() {
					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())
					Eventually(sess.Err).Should(gbytes.Say("the following is required for gitlab-auth: groups"))
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
					Eventually(sess.Err).Should(gbytes.Say("must specify --uaa-auth-client-id and --uaa-auth-client-secret to use UAA OAuth."))
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
					Eventually(sess.Err).Should(gbytes.Say("must specify --uaa-auth-client-id and --uaa-auth-client-secret to use UAA OAuth."))
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
					Eventually(sess.Err).Should(gbytes.Say("must specify --uaa-auth-cf-space to use UAA OAuth."))
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
					Eventually(sess.Err).Should(gbytes.Say("must specify --uaa-auth-auth-url, --uaa-auth-token-url and --uaa-auth-cf-url to use UAA OAuth."))
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
					Eventually(sess.Err).Should(gbytes.Say("must specify --uaa-auth-auth-url, --uaa-auth-token-url and --uaa-auth-cf-url to use UAA OAuth."))
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
					Eventually(sess.Err).Should(gbytes.Say("must specify --uaa-auth-auth-url, --uaa-auth-token-url and --uaa-auth-cf-url to use UAA OAuth."))
					Eventually(sess).Should(gexec.Exit(1))
				})
			})
		})

		Describe("generic oauth", func() {
			Context("ClientID omitted", func() {
				BeforeEach(func() {
					cmdParams = []string{"--generic-oauth-client-secret", "brock123"}
				})

				It("returns an error", func() {
					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())
					Eventually(sess.Err).Should(gbytes.Say("must specify --generic-oauth-client-id and --generic-oauth-client-secret to use Generic OAuth."))
					Eventually(sess).Should(gexec.Exit(1))
				})
			})

			Context("ClientSecret omitted", func() {
				BeforeEach(func() {
					cmdParams = []string{"--generic-oauth-client-id", "Brock Samson"}
				})

				It("returns an error", func() {
					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())
					Eventually(sess.Err).Should(gbytes.Say("must specify --generic-oauth-client-id and --generic-oauth-client-secret to use Generic OAuth."))
					Eventually(sess).Should(gexec.Exit(1))
				})
			})

			Context("display name omitted", func() {
				BeforeEach(func() {
					cmdParams = []string{
						"--generic-oauth-client-id", "Brock Samson",
						"--generic-oauth-client-secret", "brock123",
						"--generic-oauth-auth-url", "http://auth.example.url",
						"--generic-oauth-token-url", "http://token.example.url",
					}
				})

				It("returns an error", func() {
					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())
					Eventually(sess.Err).Should(gbytes.Say("must specify --generic-oauth-display-name to use Generic OAuth."))
					Eventually(sess).Should(gexec.Exit(1))
				})
			})

			Context("TokenURL omitted", func() {
				BeforeEach(func() {
					cmdParams = []string{
						"--generic-oauth-client-id", "Brock Samson",
						"--generic-oauth-client-secret", "brock123",
						"--generic-oauth-display-name", "generic oauth cool name",
						"--generic-oauth-auth-url", "http://auth.example.url",
					}
				})

				It("returns an error", func() {
					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())
					Eventually(sess.Err).Should(gbytes.Say("must specify --generic-oauth-auth-url and --generic-oauth-token-url to use Generic OAuth."))
					Eventually(sess).Should(gexec.Exit(1))
				})
			})

			Context("AuthUrl omitted", func() {
				BeforeEach(func() {
					cmdParams = []string{
						"--generic-oauth-client-id", "Brock Samson",
						"--generic-oauth-client-secret", "brock123",
						"--generic-oauth-display-name", "generic oauth cool name",
						"--generic-oauth-token-url", "http://token.example.url",
					}
				})

				It("returns an error", func() {
					sess, err := gexec.Start(flyCmd, nil, nil)
					Expect(err).ToNot(HaveOccurred())
					Eventually(sess.Err).Should(gbytes.Say("must specify --generic-oauth-auth-url and --generic-oauth-token-url to use Generic OAuth."))
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

		Context("Setting generic oauth", func() {
			BeforeEach(func() {
				cmdParams = []string{
					"--generic-oauth-client-id", "Brock Samson",
					"--generic-oauth-client-secret", "brock123",
					"--generic-oauth-auth-url", "http://auth.example.url",
					"--generic-oauth-token-url", "http://token.example.url",
					"--generic-oauth-display-name", "cool generic name",
				}
			})

			It("says 'disabled' to setting basic auth, github auth, uaa auth and 'enabled' to generic oauth", func() {
				sess, err := gexec.Start(flyCmd, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				Eventually(sess.Out).Should(gbytes.Say("Team Name: venture"))
				Eventually(sess.Out).Should(gbytes.Say("Basic Auth: disabled"))
				Eventually(sess.Out).Should(gbytes.Say("GitHub Auth: disabled"))
				Eventually(sess.Out).Should(gbytes.Say("UAA Auth: disabled"))
				Eventually(sess.Out).Should(gbytes.Say("Generic OAuth: enabled"))

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
				confirmHandlers()
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
							"auth": {
								"github": {
									"client_id": "barack samson",
									"client_secret": "barack123",
									"organizations": ["Obama, Inc", "Samson, Inc"],
									"teams": [{"organization_name": "Venture, Inc", "team_name": "venture-devs"}],
									"users": ["lisa", "frank"]
								},
								"uaa": {
									"client_id": "barack samson",
									"client_secret": "barack123",
									"auth_url": "http://uaa.auth.url",
									"token_url": "http://uaa.token.url",
									"cf_spaces": ["Obama, Inc", "Samson, Inc"],
									"cf_url": "http://cf.url",
									"cf_ca_cert": "cf-cert-contents"
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
					"--github-auth-auth-url", "http://enterprise.github.com/authorize",
					"--github-auth-token-url", "http://enterprise.github.com/token",
					"--github-auth-api-url", "http://enterprise.github.com/api",
					"--uaa-auth-client-id", "barack samson",
					"--uaa-auth-client-secret", "barack123",
					"--uaa-auth-cf-space", "Obama, Inc",
					"--uaa-auth-cf-space", "Samson, Inc",
					"--uaa-auth-auth-url", "http://uaa.auth.url",
					"--uaa-auth-token-url", "http://uaa.token.url",
					"--uaa-auth-cf-url", "http://cf.url",
					"--generic-oauth-client-id", "barack samson",
					"--generic-oauth-client-secret", "barack123",
					"--generic-oauth-auth-url", "http://goa.auth.url",
					"--generic-oauth-token-url", "http://goa.token.url",
					"--generic-oauth-display-name", "generic cool name",
					"--generic-oauth-auth-url-param", "param1:value1",
					"--generic-oauth-auth-url-param", "param2:value2",
				}
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/api/v1/teams/venture"),
						ghttp.VerifyJSON(`{
							"basic_auth": {
								"basic_auth_username": "brock obama",
								"basic_auth_password": "brock123"
							},
							"auth": {
								"github": {
									"client_id": "barack samson",
									"client_secret": "barack123",
									"organizations": ["Obama, Inc", "Samson, Inc"],
									"teams": [{"organization_name": "Venture, Inc", "team_name": "venture-devs"}],
									"users": ["lisa", "frank"],
									"auth_url": "http://enterprise.github.com/authorize",
									"token_url": "http://enterprise.github.com/token",
									"api_url": "http://enterprise.github.com/api"
								},
								"oauth": {
									"display_name": "generic cool name",
									"client_id": "barack samson",
									"client_secret": "barack123",
									"auth_url": "http://goa.auth.url",
									"auth_url_params": {
										"param1": "value1",
										"param2": "value2"
									},
									"token_url": "http://goa.token.url"
								},
								"uaa": {
									"client_id": "barack samson",
									"client_secret": "barack123",
									"auth_url": "http://uaa.auth.url",
									"token_url": "http://uaa.token.url",
									"cf_spaces": ["Obama, Inc", "Samson, Inc"],
									"cf_url": "http://cf.url"
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
